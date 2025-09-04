package model

import (
	"PProject/service/mgo"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
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
