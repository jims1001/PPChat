package message

import (
	chatmodel "PProject/module/chat/model"
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MaxSenderSeqLE 找到“由 sender 发送、且 seq ≤ upto”的最大 seq（用于外发已读 upToSeq）
func (s *Store) MaxSenderSeqLE(ctx context.Context, tenant, conv, sender string, upto int64) (int64, error) {
	cur, err := s.MsgColl.Find(ctx, bson.M{
		"tenant_id": tenant, "conversation_id": conv, "sender_id": sender,
		"seq": bson.M{"$lte": upto},
	}, options.Find().SetSort(bson.M{"seq": -1}).SetLimit(1))
	if err != nil {
		return 0, err
	}
	defer func(cur *mongo.Cursor, ctx context.Context) {
		err := cur.Close(ctx)
		if err != nil {

		}
	}(cur, ctx)
	if cur.Next(ctx) {
		var m chatmodel.MessageModel
		_ = cur.Decode(&m)
		return m.Seq, nil
	}
	return 0, nil
}

// OnPeerReadContinuous B 连续已读到 S ：给 A（发件人）推进 read_outbox_seq
func (s *Store) OnPeerReadContinuous(ctx context.Context, tenant, conv, readerUser, senderUser string, upto int64) error {
	k, err := s.MaxSenderSeqLE(ctx, tenant, conv, senderUser, upto)
	if err != nil {
		return err
	}
	if k > 0 {
		return s.BumpReadOutboxSeq(ctx, tenant, senderUser, conv, k)
	}
	return nil
}

// FilterSeqBySender B 跳读新增点：把属于 sender 的那些 seq 打包（如需 push，业务层可调用此函数的返回）
func (s *Store) FilterSeqBySender(ctx context.Context, tenant, conv, sender string, seqs []int64) ([]int64, error) {
	if len(seqs) == 0 {
		return nil, nil
	}
	cur, err := s.MsgColl.Find(ctx, bson.M{
		"tenant_id": tenant, "conversation_id": conv,
		"seq": bson.M{"$in": seqs}, "sender_id": sender,
	}, options.Find().SetProjection(bson.M{"seq": 1}))
	if err != nil {
		return nil, err
	}
	defer func(cur *mongo.Cursor, ctx context.Context) {
		err := cur.Close(ctx)
		if err != nil {

		}
	}(cur, ctx)
	var out []int64
	for cur.Next(ctx) {
		var m chatmodel.MessageModel
		_ = cur.Decode(&m)
		out = append(out, m.Seq)
	}
	return out, nil
}
