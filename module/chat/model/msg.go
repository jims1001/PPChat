package model

import (
	"PProject/service/mgo"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

//
// ============================ 枚举（与 proto 数值保持一致） ============================
//

// 会话类型
type SessionType int32

const (
	SessionTypeUnspecified SessionType = 0 // 未指定
	SINGLE_CHAT            SessionType = 1 // 单聊
	GROUP_CHAT             SessionType = 2 // 群聊
	SUPER_GROUP            SessionType = 3 // 超大群/频道
	NOTIFICATION           SessionType = 4 // 系统通知会话
	CUSTOMER               SessionType = 5 // 客服/工单会话
)

// 内容类型
type ContentType int32

const (
	ContentTypeUnspecified ContentType = 0 // 未指定

	// 文本类
	TEXT          ContentType = 101 // 文本
	ADVANCED_TEXT ContentType = 102 // 富文本
	MARKDOWN      ContentType = 103 // Markdown

	// 媒体类
	PICTURE  ContentType = 201 // 图片
	SOUND    ContentType = 202 // 语音
	VIDEO    ContentType = 203 // 视频
	FILE     ContentType = 204 // 文件
	LOCATION ContentType = 205 // 位置

	// 交互类
	CARD    ContentType = 301 // 名片
	AT_TEXT ContentType = 302 // @消息
	FACE    ContentType = 303 // 表情
	MERGE   ContentType = 304 // 合并转发
	QUOTE   ContentType = 305 // 引用
	TYPING  ContentType = 306 // 输入中
	CUSTOM  ContentType = 399 // 自定义消息

	// 系统类
	MsgNOTIFICATION ContentType = 401 // 系统通知
	REACTION        ContentType = 402 // 表情回应
	REVOKE          ContentType = 403 // 撤回消息
)

// 消息来源
type MsgFrom int32

const (
	MsgFromUnspecified MsgFrom = 0 // 未指定
	FROM_USER          MsgFrom = 1 // 普通用户
	FROM_SYSTEM        MsgFrom = 2 // 系统
	FROM_ADMIN         MsgFrom = 3 // 管理员/客服
	FROM_ROBOT         MsgFrom = 4 // 机器人
	FROM_THIRD         MsgFrom = 5 // 第三方接入
)

// 平台 ID
type PlatformID int32

const (
	PlatformUnspecified PlatformID = 0 // 未指定
	IOS                 PlatformID = 1 // iOS
	ANDROID             PlatformID = 2 // Android
	WINDOWS             PlatformID = 3 // Windows
	MAC                 PlatformID = 4 // macOS
	WEB                 PlatformID = 5 // Web/H5
	MINI                PlatformID = 6 // 小程序
	LINUX               PlatformID = 7 // Linux 桌面端
	ADMINMANAGE         PlatformID = 8 // 管理后台
	API                 PlatformID = 9 // API/服务调用
)

//
// ============================ 撤回/回执/反应 ============================
//

// 撤回信息
type RevokeModel struct {
	Role     int32  `bson:"role"`     // 操作人角色（0=普通成员,1=管理员,2=系统）
	UserID   string `bson:"user_id"`  // 操作人ID
	Nickname string `bson:"nickname"` // 操作人昵称快照
	Time     int64  `bson:"time"`     // 撤回时间 (Unix ms)
}

//
// ============================ 顶层消息模型 ============================
//

type MessageModel struct {
	ID primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`

	TenantID string `bson:"tenant_id"` // PK

	// —— 标识/时间/路由 —— //
	ClientMsgID      string      `bson:"client_msg_id,omitempty" json:"client_msg_id,omitempty"`     // 客户端生成的消息ID（幂等）
	ServerMsgID      string      `bson:"server_msg_id"           json:"server_msg_id"`               // 服务端消息ID（全局唯一）
	CreateTimeMS     int64       `bson:"create_time_ms"          json:"create_time_ms"`              // 客户端创建时间（ms）
	SendTimeMS       int64       `bson:"send_time_ms"            json:"send_time_ms"`                // 服务端投递时间（ms，权威时间）
	SessionType      SessionType `bson:"session_type"            json:"session_type"`                // 会话类型
	SendID           string      `bson:"send_id"                 json:"send_id"`                     // 发送者ID
	RecvID           string      `bson:"recv_id"                 json:"recv_id"`                     // 接收方标识（单聊=对端userID；群聊=groupID）
	MsgFrom          MsgFrom     `bson:"msg_from"                json:"msg_from"`                    // 消息来源
	ContentType      ContentType `bson:"content_type"            json:"content_type"`                // 内容类型
	SenderPlatformID PlatformID  `bson:"sender_platform_id"      json:"sender_platform_id"`          // 发送端平台
	SenderNickname   string      `bson:"sender_nickname,omitempty" json:"sender_nickname,omitempty"` // 昵称快照
	SenderFaceURL    string      `bson:"sender_face_url,omitempty" json:"sender_face_url,omitempty"` // 头像快照
	GroupID          string      `bson:"group_id,omitempty"      json:"group_id,omitempty"`          // 群ID
	ConversationID   string      `bson:"conversation_id"         json:"conversation_id"`             // 会话ID（单聊=拼接，群聊=groupID）
	Seq              int64       `bson:"seq_num"                     json:"seq_num"`                 // 序列号（严格有序）
	IsRead           int         `bson:"is_read,omitempty"       json:"is_read,omitempty"`           // 是否已读 (0/1)
	Status           int         `bson:"status,omitempty"        json:"status,omitempty"`            // 状态（0=正常，1=撤回，2=删除，3=失败）

	// —— Guild/Channel/Thread —— //
	GuildID   string `bson:"guild_id,omitempty"   json:"guild_id,omitempty"`
	ChannelID string `bson:"channel_id,omitempty" json:"channel_id,omitempty"`
	ThreadID  string `bson:"thread_id,omitempty"  json:"thread_id,omitempty"`

	// —— 文本/多态 Elem —— //
	ContentText string `bson:"content_text,omitempty" json:"content_text,omitempty"` // 文本内容（轻量）

	// —— 推送/扩展 —— //
	AttachedInfo string                 `bson:"attached_info,omitempty" json:"attached_info,omitempty"` // 附加信息（透传字符串）
	Ex           map[string]interface{} `bson:"ex,omitempty"            json:"ex,omitempty"`            // 扩展字段（业务自定义）
	LocalEx      string                 `bson:"local_ex,omitempty"      json:"local_ex,omitempty"`      // 本地扩展（仅客户端）

	// —— 协作/审计 —— //
	IsEdited     int      `bson:"is_edited,omitempty"      json:"is_edited,omitempty"`          // 是否被编辑 (0/1)
	EditedAtMS   int64    `bson:"edited_at_ms,omitempty"   json:"edited_at_ms,omitempty"`       // 编辑时间
	EditVersion  int32    `bson:"edit_version,omitempty"   json:"edit_version,omitempty"`       // 编辑版本号
	ExpireAtMS   int64    `bson:"expire_at_ms,omitempty"   json:"expire_at_ms,omitempty"`       // 过期时间（ms）
	AccessLevel  string   `bson:"access_level,omitempty"   json:"access_level,omitempty"`       // 访问级别
	TraceID      string   `bson:"trace_id,omitempty"       json:"trace_id,omitempty"`           // 链路追踪ID
	SessionTrace string   `bson:"session_trace_id,omitempty" json:"session_trace_id,omitempty"` // 会话审计ID
	Tags         []string `bson:"tags,omitempty"         json:"tags,omitempty"`                 // 标签
	ReplyTo      string   `bson:"reply_to,omitempty"     json:"reply_to,omitempty"`             // 被回复的消息ID
	IsEphemeral  int      `bson:"is_ephemeral,omitempty" json:"is_ephemeral,omitempty"`         // 是否临时消息 (0/1)

	// —— 富媒体复合体/自动审核 —— //
	Rich    map[string]interface{}   `bson:"rich,omitempty"    json:"rich,omitempty"`    // 富媒体扩展
	AutoMod []map[string]interface{} `bson:"automod,omitempty" json:"automod,omitempty"` // 自动审核信号

	// —— 撤回信息（可选） —— //
	Revoke *RevokeModel `bson:"revoke,omitempty" json:"revoke,omitempty"` // 撤回事件
}

func (sess *MessageModel) GetTableName() string {
	return "message"
}

func (sess *MessageModel) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}
