package message

import (
	chatmodel "PProject/module/chat/model"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *Store) UpsertConversationShadow(ctx context.Context, c chatmodel.Conversation) error {
	_, err := s.ConvColl.UpdateOne(ctx,
		bson.M{chatmodel.ConversationFieldTenantID: c.TenantID,
			chatmodel.ConversationFieldOwnerUserID:    c.OwnerUserID,
			chatmodel.ConversationFieldConversationID: c.ConversationID},
		bson.M{
			"$setOnInsert": bson.M{
				chatmodel.ConversationFieldTenantID:         c.TenantID,
				chatmodel.ConversationFieldOwnerUserID:      c.OwnerUserID,
				chatmodel.ConversationFieldConversationID:   c.ConversationID,
				chatmodel.ConversationFieldConversationType: c.ConversationType,
				chatmodel.ConversationFieldUserID:           c.UserID,
				chatmodel.ConversationFieldGroupID:          c.GroupID,
				chatmodel.ConversationFieldCreateTime:       time.Now(),
			},
			"$set": bson.M{
				chatmodel.ConversationFieldMinSeq:        c.MinSeq,
				chatmodel.ConversationFieldServerMaxSeq:  c.ServerMaxSeq,
				chatmodel.ConversationFieldRecvMsgOpt:    c.RecvMsgOpt,
				chatmodel.ConversationFieldIsPinned:      c.IsPinned,
				chatmodel.ConversationFieldIsPrivateChat: c.IsPrivateChat,
				chatmodel.ConversationFieldBurnDuration:  c.BurnDuration,
				chatmodel.ConversationFieldGroupAtType:   c.GroupAtType,
				chatmodel.ConversationFieldAttachedInfo:  c.AttachedInfo,
				chatmodel.ConversationFieldEx:            c.Ex,
				chatmodel.ConversationFieldUpdatedAt:     time.Now().UnixMilli(),
			},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

// MarkReadTo 连续已读：把 ≤ upToSeq 标记为已读；并返回推进后的 ReadSeq
func (s *Store) MarkReadTo(ctx context.Context, tenant, owner, conv string, upToSeq, minSeq, maxSeq int64) (int64, error) {
	tgt := clamp(upToSeq, minSeq, maxSeq)
	res := s.ConvColl.FindOneAndUpdate(ctx,
		bson.M{chatmodel.ConversationFieldTenantID: tenant,
			chatmodel.ConversationFieldOwnerUserID:    owner,
			chatmodel.ConversationFieldConversationID: conv},

		bson.M{"$max": bson.M{chatmodel.ConversationFieldReadSeq: tgt},
			"$set": bson.M{chatmodel.ConversationFieldUpdatedAt: time.Now().UnixMilli()}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	)

	var out chatmodel.Conversation
	if err := res.Decode(&out); err != nil && err != mongo.ErrNoDocuments {
		return 0, err
	}
	return out.ReadSeq, nil
}

// RefreshShadowWatermark 影子水位刷新（列表页或 push 消费时调用）
func (s *Store) RefreshShadowWatermark(ctx context.Context, tenant, owner, conv string, minSeq, maxSeq int64) error {
	_, err := s.ConvColl.UpdateOne(ctx,
		bson.M{chatmodel.ConversationFieldTenantID: tenant,
			chatmodel.ConversationFieldOwnerUserID:    owner,
			chatmodel.ConversationFieldConversationID: conv},
		bson.M{"$set": bson.M{chatmodel.ConversationFieldMinSeq: minSeq,
			chatmodel.ConversationFieldServerMaxSeq: maxSeq,
			chatmodel.ConversationFieldUpdatedAt:    time.Now().UnixMilli()}},
	)
	return err
}

// BumpReadOutboxSeq 推进对端“外发被读游标”（P2P/小群）——给 sender 这条用户态记录推进 read_outbox_seq
func (s *Store) BumpReadOutboxSeq(ctx context.Context, tenant, senderUser, conv string, upToSeq int64) error {
	_, err := s.ConvColl.UpdateOne(ctx,
		bson.M{chatmodel.ConversationFieldTenantID: tenant,
			chatmodel.ConversationFieldOwnerUserID:    senderUser,
			chatmodel.ConversationFieldConversationID: conv},
		bson.M{"$max": bson.M{chatmodel.ConversationFieldReadOutboxSeq: upToSeq},
			"$set": bson.M{chatmodel.ConversationFieldUpdatedAt: time.Now().UnixMilli()}},
	)
	return err
}
