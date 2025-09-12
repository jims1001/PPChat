package message

import (
	chatmodel "PProject/module/chat/model"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AddSparseReads 在一批 seq 上做“新增置位”，返回真正新增的 seq（用于 @ 扣减 / 双勾稀疏回执）
func (s *Store) AddSparseReads(ctx context.Context, tenant, conv, user string, seqs []int64, readSeq int64) (added []int64, err error) {
	// 过滤：仅保留 > readSeq 的点
	type bucket struct {
		start int64
		list  []int
	}
	byBlock := map[int64]*bucket{}
	for _, q := range seqs {
		if q <= readSeq {
			continue
		}
		bs, idx := bitIndex(q)
		b := byBlock[bs]
		if b == nil {
			b = &bucket{start: bs}
			byBlock[bs] = b
		}
		b.list = append(b.list, idx)
	}

	now := time.Now().UnixMilli()
	for bs, b := range byBlock {
		var doc chatmodel.ReadSparseBlock
		err := s.SparseBlockColl.FindOne(ctx, bson.M{
			chatmodel.RSBFieldTenantID:       tenant,
			chatmodel.RSBFieldConversationID: conv,
			chatmodel.RSBFieldUserID:         user,
			chatmodel.RSBFieldBlockStart:     bs,
		}).Decode(&doc)
		if err != nil {
			// 新块
			bits := make([]byte, chatmodel.BlockK/8)
			for _, idx := range b.list {
				if setBitInPlace(bits, idx) {
					seq := bs + int64(idx)
					added = append(added, seq)
				}
			}
			_, err = s.SparseBlockColl.UpdateOne(ctx,
				bson.M{chatmodel.RSBFieldTenantID: tenant,
					chatmodel.RSBFieldConversationID: conv,
					chatmodel.RSBFieldUserID:         user,
					chatmodel.RSBFieldBlockStart:     bs},
				bson.M{"$set": bson.M{chatmodel.RSBFieldBits: bits,
					chatmodel.RSBFieldUpdatedAt: now}},
				options.Update().SetUpsert(true),
			)
			if err != nil {
				return added, err
			}
		} else {
			// 旧块 merge
			bits := make([]byte, len(doc.Bits))
			copy(bits, doc.Bits)
			var localAdded []int64
			for _, idx := range b.list {
				if setBitInPlace(bits, idx) {
					localAdded = append(localAdded, bs+int64(idx))
				}
			}
			if len(localAdded) > 0 {
				_, err = s.SparseBlockColl.UpdateOne(ctx,
					bson.M{chatmodel.RSBFieldTenantID: tenant,
						chatmodel.RSBFieldConversationID: conv,
						chatmodel.RSBFieldUserID:         user,
						chatmodel.RSBFieldBlockStart:     bs},

					bson.M{"$set": bson.M{chatmodel.RSBFieldBits: bits,
						chatmodel.RSBFieldUpdatedAt: now}},
				)
				if err != nil {
					return added, err
				}
				added = append(added, localAdded...)
			} else {
				// 仅更新时间
				_, _ = s.SparseBlockColl.UpdateOne(ctx,
					bson.M{chatmodel.RSBFieldTenantID: tenant,
						chatmodel.RSBFieldConversationID: conv,
						chatmodel.RSBFieldUserID:         user,
						chatmodel.RSBFieldBlockStart:     bs},
					bson.M{"$set": bson.M{chatmodel.RSBFieldUpdatedAt: now}},
				)
			}
		}
	}
	return added, nil
}

// CompactSparseToPrefix 压实：把从 readSeq+1 起的连续 1 并到 ReadSeq，清理块前缀位
func (s *Store) CompactSparseToPrefix(ctx context.Context, tenant, owner, conv string, readSeq, maxSeq int64) (int64, error) {
	cur := readSeq + 1
	for cur <= maxSeq {
		bs, idx := bitIndex(cur)
		var doc chatmodel.ReadSparseBlock
		err := s.SparseBlockColl.FindOne(ctx, bson.M{
			chatmodel.RSBFieldTenantID:       tenant,
			chatmodel.RSBFieldConversationID: conv,
			chatmodel.RSBFieldUserID:         owner,
			chatmodel.RSBFieldBlockStart:     bs,
		}).Decode(&doc)
		if err != nil {
			break // 下个块没有，结束
		}
		// 从 idx 开始连续扫描
		end := cur - 1
		for i := idx; i < chatmodel.BlockK && (bs+int64(i)) <= maxSeq; i++ {
			byteIdx := i >> 3
			off := uint(i & 7)
			if (doc.Bits[byteIdx] & (1 << off)) == 0 { // 碰到 0，停止
				break
			}
			end = bs + int64(i)
		}
		if end >= cur {
			// 提升 read_seq
			_, err = s.ConvColl.UpdateOne(ctx,
				bson.M{chatmodel.ConversationFieldTenantID: tenant,
					chatmodel.ConversationFieldOwnerUserID:    owner,
					chatmodel.ConversationFieldConversationID: conv},
				bson.M{"$max": bson.M{chatmodel.ConversationFieldReadSeq: end},
					"$set": bson.M{chatmodel.ConversationFieldUpdatedAt: time.Now().UnixMilli()}},
			)
			if err != nil {
				return readSeq, err
			}
			// 裁剪该块 ≤end 的位
			for i := idx; (bs+int64(i)) <= end && i < chatmodel.BlockK; i++ {
				byteIdx := i >> 3
				off := uint(i & 7)
				doc.Bits[byteIdx] &^= (1 << off)
			}
			// 写回（若整块清空可考虑删除）
			_, err = s.SparseBlockColl.UpdateOne(ctx,
				bson.M{chatmodel.RSBFieldTenantID: tenant,
					chatmodel.RSBFieldConversationID: conv,
					chatmodel.RSBFieldUserID:         owner,
					chatmodel.RSBFieldBlockStart:     bs},
				bson.M{"$set": bson.M{chatmodel.RSBFieldBits: doc.Bits,
					chatmodel.RSBFieldUpdatedAt: time.Now().UnixMilli()}},
			)
			if err != nil {
				return readSeq, err
			}
			readSeq = end
			cur = end + 1
			continue
		}
		break
	}
	return readSeq, nil
}
