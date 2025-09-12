package model

import (
	"PProject/service/mgo"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

//
// ============================ 枚举（与 proto 数值保持一致） ============================
//

// 会话类型
type SessionType int32

// MessageModel collection field constants
const (
	MsgFieldID               = "_id"
	MsgFieldTenantID         = "tenant_id"
	MsgFieldClientMsgID      = "client_msg_id"
	MsgFieldServerMsgID      = "server_msg_id"
	MsgFieldCreateTimeMS     = "create_time_ms"
	MsgFieldSendTimeMS       = "send_time_ms"
	MsgFieldSessionType      = "session_type"
	MsgFieldSendID           = "send_id"
	MsgFieldRecvID           = "recv_id"
	MsgFieldMsgFrom          = "msg_from"
	MsgFieldContentType      = "content_type"
	MsgFieldSenderPlatformID = "sender_platform_id"
	MsgFieldSenderNickname   = "sender_nickname"
	MsgFieldSenderFaceURL    = "sender_face_url"
	MsgFieldGroupID          = "group_id"
	MsgFieldConversationID   = "conversation_id"
	MsgFieldSeq              = "seq_num"
	MsgFieldIsRead           = "is_read"
	MsgFieldStatus           = "status"

	// Guild/Channel/Thread
	MsgFieldGuildID   = "guild_id"
	MsgFieldChannelID = "channel_id"
	MsgFieldThreadID  = "thread_id"

	// Content elements
	MsgFieldTextElem         = "text_elem"
	MsgFieldAdvancedTextElem = "advanced_text_elem"
	MsgFieldMarkdownTextElem = "markdown_text_elem"
	MsgFieldPictureElem      = "picture_elem"
	MsgFieldSoundElem        = "sound_elem"
	MsgFieldVideoElem        = "video_elem"
	MsgFieldFileElem         = "file_elem"
	MsgFieldLocationElem     = "location_elem"
	MsgFieldCardElem         = "card_elem"
	MsgFieldAtTextElem       = "at_text_elem"
	MsgFieldFaceElem         = "face_elem"
	MsgFieldMergeElem        = "merge_elem"
	MsgFieldQuoteElem        = "quote_elem"
	MsgFieldCustomElem       = "custom_elem"
	MsgFieldNotificationElem = "notification_elem"

	// Lightweight redundancy
	MsgFieldContentText = "content_text"

	// Push / extension
	MsgFieldOfflinePush  = "offline_push"
	MsgFieldAttachedInfo = "attached_info"
	MsgFieldEx           = "ex"
	MsgFieldLocalEx      = "local_ex"

	// Collaboration / audit
	MsgFieldIsEdited     = "is_edited"
	MsgFieldEditedAtMS   = "edited_at_ms"
	MsgFieldEditVersion  = "edit_version"
	MsgFieldExpireAtMS   = "expire_at_ms"
	MsgFieldAccessLevel  = "access_level"
	MsgFieldTraceID      = "trace_id"
	MsgFieldSessionTrace = "session_trace_id"
	MsgFieldTags         = "tags"
	MsgFieldReplyTo      = "reply_to"
	MsgFieldIsEphemeral  = "is_ephemeral"

	// Rich media / automod
	MsgFieldRich    = "rich"
	MsgFieldAutoMod = "automod"

	// Revoke info
	MsgFieldRevoke = "revoke"
)

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

type TextElem struct {
	Content string `bson:"content" json:"content"`
}

type PictureBaseInfo struct {
	UUID   string `bson:"uuid,omitempty" json:"uuid,omitempty"`
	Type   string `bson:"type,omitempty" json:"type,omitempty"`
	Size   int64  `bson:"size,omitempty" json:"size,omitempty"`
	Width  int32  `bson:"width,omitempty" json:"width,omitempty"`
	Height int32  `bson:"height,omitempty" json:"height,omitempty"`
	URL    string `bson:"url,omitempty" json:"url,omitempty"`
}
type PictureElem struct {
	SourcePicture   *PictureBaseInfo `bson:"source_picture,omitempty"   json:"source_picture,omitempty"`
	BigPicture      *PictureBaseInfo `bson:"big_picture,omitempty"      json:"big_picture,omitempty"`
	SnapshotPicture *PictureBaseInfo `bson:"snapshot_picture,omitempty" json:"snapshot_picture,omitempty"`
}

type FileElem struct {
	UUID      string `bson:"uuid,omitempty" json:"uuid,omitempty"`
	SourceURL string `bson:"source_url,omitempty" json:"source_url,omitempty"`
	FileName  string `bson:"file_name,omitempty" json:"file_name,omitempty"`
	FileSize  int64  `bson:"file_size,omitempty" json:"file_size,omitempty"`
	FileType  string `bson:"file_type,omitempty" json:"file_type,omitempty"`
}

type CustomElem struct {
	Data        map[string]interface{} `bson:"data,omitempty" json:"data,omitempty"`
	Description string                 `bson:"description,omitempty" json:"description,omitempty"`
	Extension   string                 `bson:"extension,omitempty"  json:"extension,omitempty"`
}

type NotificationElem struct {
	Detail string `bson:"detail" json:"detail"` // 建议放 JSON 字符串
}

type QuoteElem struct {
	Text         string       `bson:"text,omitempty" json:"text,omitempty"`
	QuoteMessage *MessageLite `bson:"quote_message,omitempty" json:"quote_message,omitempty"`
}
type MessageLite struct {
	ServerMsgID string      `bson:"server_msg_id,omitempty" json:"server_msg_id,omitempty"`
	ContentType ContentType `bson:"content_type,omitempty"  json:"content_type,omitempty"`
	TextElem    *TextElem   `bson:"text_elem,omitempty"     json:"text_elem,omitempty"`
	SendTimeMS  int64       `bson:"send_time_ms,omitempty"  json:"send_time_ms,omitempty"`
	SendID      string      `bson:"send_id,omitempty"       json:"send_id,omitempty"`
	RecvID      string      `bson:"recv_id,omitempty"       json:"recv_id,omitempty"`
	SessionType SessionType `bson:"session_type,omitempty"  json:"session_type,omitempty"`
}

// 富文本（例如链接/加粗/高亮片段）
type AdvancedTextElem struct {
	Text              string           `bson:"text" json:"text"`                                                   // 原文
	MessageEntityList []*MessageEntity `bson:"message_entity_list,omitempty" json:"message_entity_list,omitempty"` // 片段实体
}

// Markdown 文本
type MarkdownTextElem struct {
	Content string `bson:"content" json:"content"` // Markdown 源
}

// 语音（为兼容，重复了 SoundBaseInfo 的字段）
type SoundElem struct {
	UUID      string `bson:"uuid,omitempty" json:"uuid,omitempty"`
	SoundPath string `bson:"sound_path,omitempty" json:"sound_path,omitempty"`
	SourceURL string `bson:"source_url,omitempty" json:"source_url,omitempty"`
	DataSize  int64  `bson:"data_size,omitempty" json:"data_size,omitempty"`
	Duration  int64  `bson:"duration,omitempty" json:"duration,omitempty"`     // 毫秒
	SoundType string `bson:"sound_type,omitempty" json:"sound_type,omitempty"` // 编码格式（aac/amr/opus）
}

// 视频（为兼容，重复了 VideoBaseInfo 的字段）
type VideoElem struct {
	VideoPath      string `bson:"video_path,omitempty" json:"video_path,omitempty"`
	VideoUUID      string `bson:"video_uuid,omitempty" json:"video_uuid,omitempty"`
	VideoURL       string `bson:"video_url,omitempty" json:"video_url,omitempty"`
	VideoType      string `bson:"video_type,omitempty" json:"video_type,omitempty"`
	VideoSize      int64  `bson:"video_size,omitempty" json:"video_size,omitempty"`
	Duration       int64  `bson:"duration,omitempty" json:"duration,omitempty"`
	SnapshotPath   string `bson:"snapshot_path,omitempty" json:"snapshot_path,omitempty"`
	SnapshotUUID   string `bson:"snapshot_uuid,omitempty" json:"snapshot_uuid,omitempty"`
	SnapshotSize   int64  `bson:"snapshot_size,omitempty" json:"snapshot_size,omitempty"`
	SnapshotURL    string `bson:"snapshot_url,omitempty" json:"snapshot_url,omitempty"`
	SnapshotWidth  int32  `bson:"snapshot_width,omitempty" json:"snapshot_width,omitempty"`
	SnapshotHeight int32  `bson:"snapshot_height,omitempty" json:"snapshot_height,omitempty"`
	SnapshotType   string `bson:"snapshot_type,omitempty" json:"snapshot_type,omitempty"`
}

// 位置
type LocationElem struct {
	Description string  `bson:"description,omitempty" json:"description,omitempty"` // 展示文案/POI 名称
	Longitude   float64 `bson:"longitude,omitempty" json:"longitude,omitempty"`
	Latitude    float64 `bson:"latitude,omitempty" json:"latitude,omitempty"`
}

// 名片/用户卡片
type CardElem struct {
	UserID   string `bson:"user_id,omitempty" json:"user_id,omitempty"`   // 卡片所指用户ID
	Nickname string `bson:"nickname,omitempty" json:"nickname,omitempty"` // 昵称快照
	FaceURL  string `bson:"face_url,omitempty" json:"face_url,omitempty"` // 头像快照
	Ex       string `bson:"ex,omitempty" json:"ex,omitempty"`             // 扩展 JSON
}

// AtTextElem @消息（文本 + @的对象）
type AtTextElem struct {
	Text         string       `bson:"text,omitempty" json:"text,omitempty"`
	AtUserList   []string     `bson:"at_user_list,omitempty" json:"at_user_list,omitempty"`   // 被 @ 的 userID 列表
	AtUsersInfo  []*AtInfo    `bson:"at_users_info,omitempty" json:"at_users_info,omitempty"` // @ 对象的昵称快照
	QuoteMessage *MessageLite `bson:"quote_message,omitempty" json:"quote_message,omitempty"` // 可选：引用消息
	IsAtSelf     bool         `bson:"is_at_self,omitempty" json:"is_at_self,omitempty"`       // 是否包含 @我
}

type AtInfo struct {
	AtUserID      string `bson:"at_user_id,omitempty" json:"at_user_id,omitempty"`
	GroupNickname string `bson:"group_nickname,omitempty" json:"group_nickname,omitempty"`
}

// FaceElem 表情/贴纸
type FaceElem struct {
	Index int32                  `bson:"index,omitempty" json:"index,omitempty"`
	Data  map[string]interface{} `bson:"data,omitempty" json:"data,omitempty"` // 扩展，自定义贴纸/URL
}

// 合并转发
type MergeElem struct {
	Title             string           `bson:"title,omitempty" json:"title,omitempty"`                 // 合并消息标题
	AbstractList      []string         `bson:"abstract_list,omitempty" json:"abstract_list,omitempty"` // 每条摘要
	MultiMessage      []*MessageLite   `bson:"multi_message,omitempty" json:"multi_message,omitempty"` // 嵌入的消息快照
	MessageEntityList []*MessageEntity `bson:"message_entity_list,omitempty" json:"message_entity_list,omitempty"`
}

// 富文本实体：用于在文本中标记片段（如超链接、加粗、@mention）
type MessageEntity struct {
	Type   string `bson:"type,omitempty"   json:"type,omitempty"`   // 实体类型 ("url"/"bold"/"italic"/"mention"/...)
	Offset int32  `bson:"offset,omitempty" json:"offset,omitempty"` // 起始偏移（UTF-8 rune 或字节）
	Length int32  `bson:"length,omitempty" json:"length,omitempty"` // 长度（同上单位）
	URL    string `bson:"url,omitempty"    json:"url,omitempty"`    // 链接（当 type=url 时）
	Ex     string `bson:"ex,omitempty"     json:"ex,omitempty"`     // 扩展（JSON）
}

// 离线推送配置（跨平台）
type OfflinePushInfo struct {
	Title string `bson:"title,omitempty" json:"title,omitempty"` // 推送标题
	Desc  string `bson:"desc,omitempty" json:"desc,omitempty"`   // 推送正文
	Ex    string `bson:"ex,omitempty" json:"ex,omitempty"`       // 透传扩展（JSON 字符串）

	IOSBadgeCountPlus1 bool   `bson:"ios_badge_count_plus1,omitempty" json:"ios_badge_count_plus1,omitempty"` // iOS 角标是否 +1
	IOSCategory        string `bson:"ios_category,omitempty"          json:"ios_category,omitempty"`          // iOS 通知分类
	IosSound           string `bson:"ios_sound,omitempty"             json:"ios_sound,omitempty"`             // iOS 自定义提示音

	AndroidVivoClassification bool `bson:"android_vivo_classification,omitempty" json:"android_vivo_classification,omitempty"` // vivo 分类
}

// ============================ 顶层消息模型 ============================
//

type MessageModel struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	TenantID string             `bson:"tenant_id"`

	// —— 标识/时间/路由 —— //
	ClientMsgID      string      `bson:"client_msg_id,omitempty" json:"client_msg_id,omitempty"`
	ServerMsgID      string      `bson:"server_msg_id"           json:"server_msg_id"`
	CreateTimeMS     int64       `bson:"create_time_ms"          json:"create_time_ms"`
	SendTimeMS       int64       `bson:"send_time_ms"            json:"send_time_ms"`
	SessionType      SessionType `bson:"session_type"            json:"session_type"`
	SendID           string      `bson:"send_id"                 json:"send_id"`
	RecvID           string      `bson:"recv_id"                 json:"recv_id"`
	MsgFrom          MsgFrom     `bson:"msg_from"                json:"msg_from"`
	ContentType      ContentType `bson:"content_type"            json:"content_type"`
	SenderPlatformID PlatformID  `bson:"sender_platform_id"      json:"sender_platform_id"`
	SenderNickname   string      `bson:"sender_nickname,omitempty" json:"sender_nickname,omitempty"`
	SenderFaceURL    string      `bson:"sender_face_url,omitempty" json:"sender_face_url,omitempty"`
	GroupID          string      `bson:"group_id,omitempty"      json:"group_id,omitempty"`
	ConversationID   string      `bson:"conversation_id"         json:"conversation_id"`
	Seq              int64       `bson:"seq_num"                 json:"seq_num"`
	IsRead           int         `bson:"is_read,omitempty"       json:"is_read,omitempty"`
	Status           int         `bson:"status,omitempty"        json:"status,omitempty"`

	// —— Guild/Channel/Thread —— //
	GuildID   string `bson:"guild_id,omitempty"   json:"guild_id,omitempty"`
	ChannelID string `bson:"channel_id,omitempty" json:"channel_id,omitempty"`
	ThreadID  string `bson:"thread_id,omitempty"  json:"thread_id,omitempty"`

	// —— “内容 one-of” 可选子文档 —— //
	// 只根据 content_type 填其中一个；其余保持 nil，不会落库
	TextElem         *TextElem         `bson:"text_elem,omitempty"          json:"text_elem,omitempty"`
	AdvancedTextElem *AdvancedTextElem `bson:"advanced_text_elem,omitempty" json:"advanced_text_elem,omitempty"`
	MarkdownTextElem *MarkdownTextElem `bson:"markdown_text_elem,omitempty" json:"markdown_text_elem,omitempty"`
	PictureElem      *PictureElem      `bson:"picture_elem,omitempty"       json:"picture_elem,omitempty"`
	SoundElem        *SoundElem        `bson:"sound_elem,omitempty"         json:"sound_elem,omitempty"`
	VideoElem        *VideoElem        `bson:"video_elem,omitempty"         json:"video_elem,omitempty"`
	FileElem         *FileElem         `bson:"file_elem,omitempty"          json:"file_elem,omitempty"`
	LocationElem     *LocationElem     `bson:"location_elem,omitempty"      json:"location_elem,omitempty"`
	CardElem         *CardElem         `bson:"card_elem,omitempty"          json:"card_elem,omitempty"`
	AtTextElem       *AtTextElem       `bson:"at_text_elem,omitempty"       json:"at_text_elem,omitempty"`
	FaceElem         *FaceElem         `bson:"face_elem,omitempty"          json:"face_elem,omitempty"`
	MergeElem        *MergeElem        `bson:"merge_elem,omitempty"         json:"merge_elem,omitempty"`
	QuoteElem        *QuoteElem        `bson:"quote_elem,omitempty"         json:"quote_elem,omitempty"`
	CustomElem       *CustomElem       `bson:"custom_elem,omitempty"        json:"custom_elem,omitempty"`
	NotificationElem *NotificationElem `bson:"notification_elem,omitempty"  json:"notification_elem,omitempty"`

	// —— 兼容你现有的轻量文本冗余 —— //
	ContentText string `bson:"content_text,omitempty" json:"content_text,omitempty"`

	// —— 推送/扩展 —— //
	OfflinePush  *OfflinePushInfo       `bson:"offline_push,omitempty" json:"offline_push,omitempty"`
	AttachedInfo string                 `bson:"attached_info,omitempty" json:"attached_info,omitempty"`
	Ex           map[string]interface{} `bson:"ex,omitempty"            json:"ex,omitempty"`
	LocalEx      string                 `bson:"local_ex,omitempty"      json:"local_ex,omitempty"`

	// —— 协作/审计 —— //
	IsEdited     int      `bson:"is_edited,omitempty"      json:"is_edited,omitempty"`
	EditedAtMS   int64    `bson:"edited_at_ms,omitempty"   json:"edited_at_ms,omitempty"`
	EditVersion  int32    `bson:"edit_version,omitempty"   json:"edit_version,omitempty"`
	ExpireAtMS   int64    `bson:"expire_at_ms,omitempty"   json:"expire_at_ms,omitempty"`
	AccessLevel  string   `bson:"access_level,omitempty"   json:"access_level,omitempty"`
	TraceID      string   `bson:"trace_id,omitempty"       json:"trace_id,omitempty"`
	SessionTrace string   `bson:"session_trace_id,omitempty" json:"session_trace_id,omitempty"`
	Tags         []string `bson:"tags,omitempty"           json:"tags,omitempty"`
	ReplyTo      string   `bson:"reply_to,omitempty"       json:"reply_to,omitempty"`
	IsEphemeral  int      `bson:"is_ephemeral,omitempty"   json:"is_ephemeral,omitempty"`

	// —— 富媒体复合体/自动审核 —— //
	Rich    map[string]interface{}   `bson:"rich,omitempty"    json:"rich,omitempty"`
	AutoMod []map[string]interface{} `bson:"automod,omitempty" json:"automod,omitempty"`

	// —— 撤回信息（可选） —— //
	Revoke *RevokeModel `bson:"revoke,omitempty" json:"revoke,omitempty"`
}

func (sess *MessageModel) GetTableName() string {
	return "message"
}

func (sess *MessageModel) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

func InsertMessage(ctx context.Context, msg *MessageModel) error {

	// 设置ID和时间
	if msg.ID.IsZero() {
		msg.ID = primitive.NewObjectID()
	}
	if msg.SendTimeMS == 0 {
		msg.SendTimeMS = time.Now().UnixMilli()
	}

	// 插入
	_, err := msg.Collection().InsertOne(ctx, msg)
	if mongo.IsDuplicateKeyError(err) {
		return fmt.Errorf("seq 已存在，拒绝重复插入: %w", err)
	}
	return err
}
