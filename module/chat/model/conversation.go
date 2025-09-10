package model

import (
	"PProject/service/mgo"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Conversation 表示用户与某个会话（单聊/群聊）的本地配置与状态
type Conversation struct {
	TenantID         string `bson:"tenant_id"`          // PK
	OwnerUserID      string `bson:"owner_user_id"`      // 会话归属用户ID（谁的会话列表）
	ConversationID   string `bson:"conversation_id"`    // 会话ID（规则：单聊=userID，群聊=groupID，可拼接生成）
	ConversationType int32  `bson:"conversation_type"`  // 会话类型（1=单聊，2=群聊，3=系统通知）
	UserID           string `bson:"user_id,omitempty"`  // 单聊对象的用户ID（仅单聊有效）
	GroupID          string `bson:"group_id,omitempty"` // 群ID（仅群聊有效）

	RecvMsgOpt    int32 `bson:"recv_msg_opt"`    // 消息接收选项（0=接收并提醒，1=接收不提醒，2=屏蔽消息）
	IsPinned      bool  `bson:"is_pinned"`       // 是否置顶
	IsPrivateChat bool  `bson:"is_private_chat"` // 是否开启私密聊天（如阅后即焚模式）
	BurnDuration  int32 `bson:"burn_duration"`   // 阅后即焚时长（秒）
	GroupAtType   int32 `bson:"group_at_type"`   // 群聊@类型（0=无，1=有人@我，2=@所有人）

	AttachedInfo string `bson:"attached_info"` // 附加信息（JSON存储，例如草稿、免打扰设置等）
	Ex           string `bson:"ex"`            // 预留扩展字段

	MaxSeq int64 `bson:"max_seq"` // 已读最大消息序列（本地）
	MinSeq int64 `bson:"min_seq"` // 最小已同步序列（清理历史消息时用）

	// 保持同步 查询的时候 就不需要连表查询了
	ServerMaxSeq int64 `bson:"server_max_seq,omitempty"` // 影子= SeqConversation.MaxSeq

	CreateTime            time.Time `bson:"create_time"`              // 会话创建时间
	IsMsgDestruct         bool      `bson:"is_msg_destruct"`          // 是否开启消息销毁（按时删除）
	MsgDestructTime       int64     `bson:"msg_destruct_time"`        // 消息销毁时间（单位：秒，例如30s后销毁）
	LatestMsgDestructTime time.Time `bson:"latest_msg_destruct_time"` // 最近一条消息的销毁时间点
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
