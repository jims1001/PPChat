package message

import (
	chatmodel "PProject/module/chat/model"
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

type ConvState struct {
	MinSeq, MaxSeq int64
	ReadSeq        int64
	Unread         int64
	MentionUnread  int32
	ReadOutboxSeq  int64
}

func (s *Store) GetConvState(ctx context.Context, tenant, owner, conv string) (*ConvState, error) {
	var c chatmodel.Conversation
	if err := s.ConvColl.FindOne(ctx, bson.M{
		chatmodel.ConversationFieldTenantID:       tenant,
		chatmodel.ConversationFieldOwnerUserID:    owner,
		chatmodel.ConversationFieldConversationID: conv,
	}).Decode(&c); err != nil {
		return nil, err
	}
	// 保守未读（O(1)）
	rs := c.ReadSeq
	if rs < c.MinSeq {
		rs = c.MinSeq
	}
	unread := c.ServerMaxSeq - rs
	if unread < 0 {
		unread = 0
	}

	return &ConvState{
		MinSeq: c.MinSeq, MaxSeq: c.ServerMaxSeq,
		ReadSeq: c.ReadSeq, Unread: unread,
		MentionUnread: c.MentionUnread,
		ReadOutboxSeq: c.ReadOutboxSeq,
	}, nil
}
