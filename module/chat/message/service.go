package message

import (
	chatmodel "PProject/module/chat/model"
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

// ReadTo 连续已读入口：推进 ReadSeq + 扣@ +（P2P 可推进对端外发回执）
func (s *Store) ReadTo(ctx context.Context, tenant, owner, conv string, upto int64, minSeq, maxSeq int64,
	p2pPeerUserID string, // 仅 P2P 传对端 userID；群聊留空
) (newReadSeq int64, err error) {
	// 1) 连续推进
	newReadSeq, err = s.MarkReadTo(ctx, tenant, owner, conv, upto, minSeq, maxSeq)
	if err != nil {
		return 0, err
	}

	// 2) 扣 @ 区间
	var cv chatmodel.Conversation
	if err = s.ConvColl.FindOne(ctx, bson.M{"tenant_id": tenant, "owner_user_id": owner, "conversation_id": conv}).Decode(&cv); err == nil {
		_ = s.DeductMentionsByRange(ctx, tenant, owner, conv, cv.MentionReadSeq, newReadSeq)
	}

	// 3) P2P 外发回执（让对端看到双勾 upto）
	if p2pPeerUserID != "" {
		_ = s.OnPeerReadContinuous(ctx, tenant, conv, owner, p2pPeerUserID, newReadSeq)
	}
	return newReadSeq, nil
}

// ReadSparse 跳读入口：置位稀疏 + 扣本次新增@ + （可选）P2P对发件人推稀疏回执
func (s *Store) ReadSparse(ctx context.Context, tenant, owner, conv string, seqs []int64,
	readSeq, minSeq, maxSeq int64,
	p2pPeerUserID string, // 仅 P2P 传对端 userID；群聊留空
) (added []int64, err error) {
	// 1) 稀疏置位（返回新增位）
	added, err = s.AddSparseReads(ctx, tenant, conv, owner, seqs, readSeq)
	if err != nil {
		return nil, err
	}
	if len(added) == 0 {
		return nil, nil
	}

	// 2) 扣 @（只扣新增置位）
	_ = s.DeductMentionsBySeqs(ctx, tenant, owner, conv, added)

	// 3) P2P：把属于对端发送的那些 seq 打包（你可在业务层 push 给对端做“稀疏双勾”）
	if p2pPeerUserID != "" {
		if fromPeer, _ := s.FilterSeqBySender(ctx, tenant, conv, p2pPeerUserID, added); len(fromPeer) > 0 {
			// pushUpdateReadOutboxSparse(owner->peer, conv, fromPeer) // 业务层自实现
			_ = fromPeer // 占位
		}
	}
	return added, nil
}
