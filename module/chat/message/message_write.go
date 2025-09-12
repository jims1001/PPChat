package message

import (
	chatmodel "PProject/module/chat/model"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func (s *Store) InsertMessageCommitted(ctx context.Context, m chatmodel.MessageModel, mentioned []string) error {
	// 1) 写消息
	_, err := s.MsgColl.InsertOne(ctx, m)
	if err != nil {
		return err
	}

	// 2) 推进会话级 MaxSeq
	if err := s.BumpMaxSeq(ctx, m.TenantID, m.ConversationID, m.Seq); err != nil {
		return err
	}

	// 3) 写 @ 倒排（可选）
	if len(mentioned) > 0 {
		if err := s.AddMentions(ctx, m.TenantID, m.ConversationID, mentioned, m.Seq, "mention"); err != nil {
			return err
		}
	}

	// 4)（可选）刷新发送者/接收者的影子水位，或在 push/拉列表时批量刷新
	//    s.RefreshShadowWatermark(...)

	_ = time.Now()
	return nil
}

// OnMinSeqAdvanced minSeq 提升时（清理/TTL），对用户状态做重基线（read_seq=max(read_seq,minSeq)）并裁剪前缀块
func (s *Store) OnMinSeqAdvanced(ctx context.Context, tenant, conv string, newMin int64) error {
	// 1) 对所有该会话用户：read_seq < newMin 的对齐
	_, _ = s.ConvColl.UpdateMany(ctx,
		bson.M{"tenant_id": tenant, "conversation_id": conv, "read_seq": bson.M{"$lt": newMin}},
		bson.M{"$set": bson.M{"read_seq": newMin, "min_seq": newMin, "updated_at": time.Now().UnixMilli()}},
	)
	// 2) 裁剪稀疏块（完全在 newMin 之前的块可删；边界块可忽略或后台慢慢裁）
	_, _ = s.SparseBlockColl.DeleteMany(ctx, bson.M{
		"tenant_id": tenant, "conversation_id": conv, "block_start": bson.M{"$lt": blockStartOf(newMin)},
	})
	return nil
}
