package msgflow

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Handler struct {
	DB     DB
	Idx    *ClientMsgIndex
	Seq    *SeqAllocator
	SIDGen ServerIDGenerator
}

func NewHandler(db DB, rdb redis.UniversalClient, sidGen ServerIDGenerator) *Handler {
	return &Handler{
		DB:     db,
		Idx:    NewClientMsgIndex(rdb),
		Seq:    NewSeqAllocator(rdb, db),
		SIDGen: sidGen,
	}
}

// SaveMessage：占位→seq→落库→(seq 冲突矫正) / (sid 冲突换 sid)→提交→ACK
func (h *Handler) SaveMessage(ctx context.Context, tenant, convID, sender, clientMsgID string, body []byte) (*MessageMeta, error) {
	ph := HashPayload(body)

	// 1) 幂等占位（PENDING）
	iv, existed, err := h.Idx.EnsureEx(ctx, tenant, sender, clientMsgID, ph, h.SIDGen.New())
	if err != nil {
		return nil, err
	}
	if existed {
		// 内容校验
		if iv.PayloadHash != ph {
			return nil, fmt.Errorf("clientMsgID reused with different payload")
		}
		// 已提交：查 DB 返回
		if iv.Status == "COMMITTED" {
			if meta, _ := h.DB.FindByClientID(ctx, tenant, sender, clientMsgID); meta != nil {
				return meta, nil
			}
		} else {
			// PENDING：补查
			if meta, _ := h.DB.FindByClientID(ctx, tenant, sender, clientMsgID); meta != nil {
				_ = h.Idx.MarkCommitted(ctx, tenant, sender, clientMsgID, meta.ServerMsgID, ph)
				return meta, nil
			}
		}
	}

	// 2) 分配 seq（首条自动初始化）
	seq, err := h.Seq.NextSeq(ctx, tenant, convID)
	if err != nil {
		_ = h.Idx.RollbackShortTTL(ctx, tenant, sender, clientMsgID)
		return nil, err
	}

	// 3) 组织消息
	sid := iv.ServerMsgID
	if sid == "" {
		sid = h.SIDGen.New()
	}
	msg := &Message{
		TenantID: tenant, ConvID: convID, SenderID: sender,
		ClientMsgID: clientMsgID, ServerMsgID: sid, Seq: seq,
		PayloadHash: ph, Body: body, CreatedAtMS: time.Now().UnixMilli(),
	}

	// 4) 落库 + 冲突处理（重试）
	const maxRetry = 3
	backoff := 50 * time.Millisecond
	for i := 0; i <= maxRetry; i++ {
		err = h.DB.InsertMessage(ctx, msg)
		if err == nil {
			_ = h.Idx.MarkCommitted(ctx, tenant, sender, clientMsgID, msg.ServerMsgID, ph)
			// TODO: Outbox 投递（生产建议开启）
			return &MessageMeta{ServerMsgID: msg.ServerMsgID, Seq: msg.Seq, CreatedAtMS: msg.CreatedAtMS}, nil
		}

		// (1) clientMsgID 唯一：幂等命中
		if h.DB.IsUniqueClientIDErr(err) {
			if meta, e := h.DB.FindByClientID(ctx, tenant, sender, clientMsgID); e == nil && meta != nil {
				_ = h.Idx.MarkCommitted(ctx, tenant, sender, clientMsgID, meta.ServerMsgID, ph)
				return meta, nil
			}
		}
		// (2) server_msg_id 唯一：PENDING 阶段换 sid
		if h.DB.IsUniqueServerIDErr(err) {
			newSID := h.SIDGen.New()
			if e := h.Idx.UpdateSIDIfPending(ctx, tenant, sender, clientMsgID, ph, newSID); e != nil {
				if meta, e2 := h.DB.FindByClientID(ctx, tenant, sender, clientMsgID); e2 == nil && meta != nil {
					_ = h.Idx.MarkCommitted(ctx, tenant, sender, clientMsgID, meta.ServerMsgID, ph)
					return meta, nil
				}
				return nil, fmt.Errorf("sid collision and index update failed: %w", e)
			}
			msg.ServerMsgID = newSID
			continue
		}
		// (3) seq 唯一：Redis 落后 → 矫正到 dbMax 后取新号
		if h.DB.IsUniqueSeqErr(err) {
			if dbMax, e := h.DB.QueryMaxSeq(ctx, tenant, convID); e == nil {
				if newSeq, e2 := h.Seq.ReconcileAndNext(ctx, tenant, convID, dbMax); e2 == nil {
					msg.Seq = newSeq
					continue
				}
			}
		}
		// (4) 瞬时错误：退避
		if h.DB.IsTransientErr(err) && i < maxRetry {
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		break
	}

	_ = h.Idx.RollbackShortTTL(ctx, tenant, sender, clientMsgID)
	return nil, fmt.Errorf("insert message failed after retries: %w", err)
}
