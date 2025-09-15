package model

import (
	"PProject/service/mgo"
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Conversation collection field constants
const (
	ConversationFieldTenantID              = "tenant_id"
	ConversationFieldOwnerUserID           = "owner_user_id"
	ConversationFieldConversationID        = "conversation_id"
	ConversationFieldConversationType      = "conversation_type"
	ConversationFieldUserID                = "user_id"
	ConversationFieldGroupID               = "group_id"
	ConversationFieldRecvMsgOpt            = "recv_msg_opt"
	ConversationFieldIsPinned              = "is_pinned"
	ConversationFieldIsPrivateChat         = "is_private_chat"
	ConversationFieldBurnDuration          = "burn_duration"
	ConversationFieldGroupAtType           = "group_at_type"
	ConversationFieldAttachedInfo          = "attached_info"
	ConversationFieldEx                    = "ex"
	ConversationFieldReadSeq               = "read_seq"
	ConversationFieldReadOutboxSeq         = "read_outbox_seq"
	ConversationFieldLocalMaxSeq           = "local_max_seq"
	ConversationFieldMentionUnread         = "mention_unread"
	ConversationFieldMentionReadSeq        = "mention_read_seq"
	ConversationFieldMinSeq                = "min_seq"
	ConversationFieldServerMaxSeq          = "server_max_seq"
	ConversationFieldPerDeviceReadSeq      = "per_device_read_seq"
	ConversationFieldCreateTime            = "create_time"
	ConversationFieldUpdatedAt             = "updated_at"
	ConversationFieldIsMsgDestruct         = "is_msg_destruct"
	ConversationFieldMsgDestructTime       = "msg_destruct_time"
	ConversationFieldLatestMsgDestructTime = "latest_msg_destruct_time"
)

const (
	BlockK = 256 // 稀疏块大小（每块 256 条）
)

// Conversation 表示用户与某个会话（单聊/群聊）的本地配置与状态
type Conversation struct {
	TenantID         string `bson:"tenant_id"`
	OwnerUserID      string `bson:"owner_user_id"`
	ConversationID   string `bson:"conversation_id"`
	ConversationType int32  `bson:"conversation_type"`
	UserID           string `bson:"user_id,omitempty"`
	GroupID          string `bson:"group_id,omitempty"`

	RecvMsgOpt    int32  `bson:"recv_msg_opt"`
	IsPinned      bool   `bson:"is_pinned"`
	IsPrivateChat bool   `bson:"is_private_chat"`
	BurnDuration  int32  `bson:"burn_duration"`
	GroupAtType   int32  `bson:"group_at_type"`
	AttachedInfo  string `bson:"attached_info"`
	Ex            string `bson:"ex"`

	// —— 已读/计数 —— //
	ReadSeq       int64 `bson:"read_seq"`                  // 连续前缀（≤它全已读）
	ReadOutboxSeq int64 `bson:"read_outbox_seq,omitempty"` // 我外发被对端读到的最大seq（P2P/小群）
	LocalMaxSeq   int64 `bson:"local_max_seq,omitempty"`   // 客户端已拉到的最大seq（可选）

	// —— @计数/游标（可选缓存）—— //
	MentionUnread  int32 `bson:"mention_unread,omitempty"`
	MentionReadSeq int64 `bson:"mention_read_seq,omitempty"`

	// —— 水位影子（减少连表）—— //
	MinSeq       int64 `bson:"min_seq,omitempty"`        // = seq_conversation.min_seq
	ServerMaxSeq int64 `bson:"server_max_seq,omitempty"` // = seq_conversation.max_seq

	// —— 多端（可选）—— //
	PerDeviceReadSeq map[string]int64 `bson:"per_device_read_seq,omitempty"`

	CreateTime            time.Time `bson:"create_time"`
	UpdatedAt             time.Time `bson:"updated_at"`
	IsMsgDestruct         bool      `bson:"is_msg_destruct"`
	MsgDestructTime       time.Time `bson:"msg_destruct_time"`
	LatestMsgDestructTime time.Time `bson:"latest_msg_destruct_time"`
}

func (sess *Conversation) GetTableName() string {
	return "conversation"
}

func (sess *Conversation) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// UpdateMaxSeq 更新最大的
func (sess *Conversation) UpdateMaxSeq(ctx context.Context, tenantID, conversationID string, newMax int64) (updated bool, err error) {
	if newMax < 0 {
		return false, nil
	}
	filter := bson.M{
		"tenant_id":       tenantID,
		"conversation_id": conversationID,
		"max_seq":         bson.M{"$lt": newMax}, // 只在变大时更新，避免写放大
	}
	update := bson.M{
		"$set": bson.M{"max_seq": newMax},
	}
	res, err := sess.Collection().UpdateOne(ctx, filter, update, options.Update())
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

func (sess *Conversation) UpdateServerMaxSeq(ctx context.Context, tenantID, conversationID string, ownerUserID string, newMax int64) (updated bool, err error) {
	if newMax < 0 {
		return false, nil
	}

	filter := bson.M{
		"tenant_id":       tenantID,
		"owner_user_id":   ownerUserID, // ★ 新增条件，确保只更新该用户
		"conversation_id": conversationID,
		"server_max_seq":  bson.M{"$lt": newMax}, // 只在变大时更新，避免写放大
	}
	update := bson.M{
		"$set": bson.M{"server_max_seq": newMax},
	}
	res, err := sess.Collection().UpdateOne(ctx, filter, update, options.Update())
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

// UpdateMinSeq 更新最小的
func (sess *Conversation) UpdateMinSeq(ctx context.Context, tenantID, conversationID string, newMin int64) (updated bool, err error) {
	if newMin < 0 {
		return false, nil
	}
	filter := bson.M{
		"tenant_id":       tenantID,
		"conversation_id": conversationID,
		"min_seq":         bson.M{"$lt": newMin}, // 只在变大时更新
	}
	update := bson.M{
		"$set": bson.M{
			"min_seq": newMin,
		},
		"$max": bson.M{
			"max_seq": newMin, // 如当前 max_seq < newMin，则同步抬升，避免 min>max
		},
	}
	res, err := sess.Collection().UpdateOne(ctx, filter, update, options.Update())
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

// GetConversationByID 根据 TenantID + ConversationID 查询
func (sess *Conversation) GetConversationByID(ctx context.Context, tenantID, conversationID string) (*Conversation, error) {
	coll := sess.Collection()

	filter := bson.M{
		"tenant_id":       tenantID,
		"conversation_id": conversationID,
	}

	var conv Conversation
	err := coll.FindOne(ctx, filter).Decode(&conv)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // 没找到
		}
		return nil, err
	}
	return &conv, nil
}
