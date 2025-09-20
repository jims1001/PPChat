package model

import (
	"PProject/service/mgo"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Inbox {
// "tenant_id": "tenant001",
// "account_id": "135774",
// "name": "家具销售",
// "domain": "www.jiaju.com",
// "avatar_url": "https://cdn.xxx/logo.png",
// "welcome_title": "你好！",
// "welcome_message": "如有疑问，请联系我们",
// "widget_color": "#0099FF",
// "enable_greeting_msg": true,
// "greeting_delay_sec": 5,
// "show_reply_time_hint": true,
// "enable_email_collect": true,
// "allow_post_resolve_msg": true,
// "enable_email_thread": true,
// "help_center_id": "help_001",
// "feature_file_picker": true,
// "feature_emoji_picker": true,
// "feature_end_conversation": true,
// "feature_bot_identity": false,
// "sender_style": "friendly",
// "business_name": "",
// "created_at": "2025-09-20T13:00:00Z",
// "updated_at": "2025-09-20T13:00:00Z"
// }
// db.inbox.createIndex({ tenant_id: 1, account_id: 1, domain: 1 }, { unique: true, name: "uniq_domain" })
// db.inbox.createIndex({ tenant_id: 1, account_id: 1, name: 1 })
// Inbox 收件箱/渠道配置
type Inbox struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"       json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"           json:"tenant_id"`
	AccountID string             `bson:"account_id"          json:"account_id"`

	// —— 基本信息 —— //
	Name      string `bson:"name"        json:"name"`       // 网站名称：家具销售
	Domain    string `bson:"domain"      json:"domain"`     // www.jiaju.com
	AvatarURL string `bson:"avatar_url"  json:"avatar_url"` // 渠道头像

	// —— 欢迎语 —— //
	WelcomeTitle   string `bson:"welcome_title,omitempty"   json:"welcome_title,omitempty"`
	WelcomeMessage string `bson:"welcome_message,omitempty" json:"welcome_message,omitempty"`

	// —— UI 外观 —— //
	WidgetColor string `bson:"widget_color" json:"widget_color"` // 小部件颜色，#RRGGBB

	// —— 功能开关 —— //
	EnableGreetingMsg   bool `bson:"enable_greeting_msg"   json:"enable_greeting_msg"`
	GreetingDelaySec    int  `bson:"greeting_delay_sec"    json:"greeting_delay_sec"` // 候问消息延迟
	ShowReplyTimeHint   bool `bson:"show_reply_time_hint"  json:"show_reply_time_hint"`
	EnableEmailCollect  bool `bson:"enable_email_collect"  json:"enable_email_collect"`
	AllowPostResolveMsg bool `bson:"allow_post_resolve_msg" json:"allow_post_resolve_msg"`
	EnableEmailThread   bool `bson:"enable_email_thread"   json:"enable_email_thread"`

	// —— 帮助中心 —— //
	HelpCenterID string `bson:"help_center_id,omitempty"    json:"help_center_id,omitempty"`

	// —— 特性 —— //
	FeatureFilePicker      bool `bson:"feature_file_picker"     json:"feature_file_picker"`
	FeatureEmojiPicker     bool `bson:"feature_emoji_picker"    json:"feature_emoji_picker"`
	FeatureEndConversation bool `bson:"feature_end_conversation" json:"feature_end_conversation"`
	FeatureBotIdentity     bool `bson:"feature_bot_identity"    json:"feature_bot_identity"`

	// —— 发件人身份 —— //
	SenderStyle  string `bson:"sender_style" json:"sender_style"`                       // friendly | professional
	BusinessName string `bson:"business_name,omitempty" json:"business_name,omitempty"` // 专业模式时用

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

func (sess *Inbox) GetTableName() string {
	return "chatbox_inbox"
}

func (sess *Inbox) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// InboxAgent
//[
//{
//"tenant_id": "tenant001",
//"account_id": "135774",
//"inbox_id": "inbox001",
//"agent_id": "si",
//"auto_assignable": true,
//"created_at": "2025-09-20T14:00:00Z"
//},
//{
//"tenant_id": "tenant001",
//"account_id": "135774",
//"inbox_id": "inbox001",
//"agent_id": "客服1",
//"auto_assignable": true,
//"created_at": "2025-09-20T14:00:00Z"
//}
//]
//
//// InboxRoutingSetting
//{
//"tenant_id": "tenant001",
//"account_id": "135774",
//"inbox_id": "inbox001",
//"enable_auto_assign": true,
//"max_concurrent": 5,
//"strategy": "round_robin",
//"updated_at": "2025-09-20T14:00:00Z"
//}
//
//// 一个收件箱下，每个 Agent 唯一
//db.inbox_agent.createIndex({ tenant_id:1, account_id:1, inbox_id:1, agent_id:1 }, { unique:true })
//
//// 收件箱级设置唯一
//db.inbox_routing_setting.createIndex({ tenant_id:1, account_id:1, inbox_id:1 }, { unique:true })

// InboxAgent 收件箱与客服代理的绑定
type InboxAgent struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"   json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"       json:"tenant_id"`
	AccountID string             `bson:"account_id"      json:"account_id"`
	InboxID   string             `bson:"inbox_id"        json:"inbox_id"`
	AgentID   string             `bson:"agent_id"        json:"agent_id"`

	// 是否参与自动分配（允许个体关闭）
	AutoAssignable bool `bson:"auto_assignable" json:"auto_assignable"`

	CreatedAt time.Time `bson:"created_at"      json:"created_at"`
}

func (sess *InboxAgent) GetTableName() string {
	return "chatbox_inbox_agent"
}

func (sess *InboxAgent) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// InboxRoutingSetting 收件箱的对话分配策略
type InboxRoutingSetting struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"   json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"       json:"tenant_id"`
	AccountID string             `bson:"account_id"      json:"account_id"`
	InboxID   string             `bson:"inbox_id"        json:"inbox_id"`

	EnableAutoAssign bool `bson:"enable_auto_assign" json:"enable_auto_assign"`
	MaxConcurrent    int  `bson:"max_concurrent"     json:"max_concurrent"` // 每个代理最大并发数 (0=无限)

	// 可扩展：分配算法 round_robin / least_load / random
	Strategy string `bson:"strategy" json:"strategy"`

	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

func (sess *InboxRoutingSetting) GetTableName() string {
	return "chatbox_inbox_routing_setting"
}

func (sess *InboxRoutingSetting) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// InboxAvailability {
// "tenant_id": "tenant001",
// "account_id": "135774",
// "inbox_id": "inbox001",
// "enabled": true,
// "ooo_message": "我们的客服当前不在线，请留言。",
// "timezone": "America/Los_Angeles",
// "weekly_slots": [
// { "day_of_week": 0, "enabled": false, "all_day": false, "slots": [] }, // Sunday
// { "day_of_week": 1, "enabled": true, "all_day": false, "slots": [ { "start": "09:00", "end": "17:00" } ] },
// { "day_of_week": 2, "enabled": true, "all_day": false, "slots": [ { "start": "09:00", "end": "17:00" } ] },
// { "day_of_week": 3, "enabled": true, "all_day": false, "slots": [ { "start": "09:00", "end": "17:00" } ] },
// { "day_of_week": 4, "enabled": true, "all_day": false, "slots": [ { "start": "09:00", "end": "17:00" } ] },
// { "day_of_week": 5, "enabled": true, "all_day": false, "slots": [ { "start": "09:00", "end": "17:00" } ] },
// { "day_of_week": 6, "enabled": false, "all_day": false, "slots": [] }  // Saturday
// ],
// "updated_at": "2025-09-20T15:00:00Z"
// }
// InboxAvailability 收件箱营业时间设置
type InboxAvailability struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"        json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"            json:"tenant_id"`
	AccountID string             `bson:"account_id"           json:"account_id"`
	InboxID   string             `bson:"inbox_id"             json:"inbox_id"`

	Enabled    bool   `bson:"enabled"             json:"enabled"`               // 是否启用营业时间
	OOOMessage string `bson:"ooo_message"         json:"ooo_message,omitempty"` // 不可用时给客户的提示

	Timezone string `bson:"timezone"            json:"timezone"` // 时区，如 "America/Los_Angeles"

	WeeklySlots []DayAvailability `bson:"weekly_slots" json:"weekly_slots"` // 每周的可用时段列表

	UpdatedAt time.Time `bson:"updated_at"          json:"updated_at"`
}

func (sess *InboxAvailability) GetTableName() string {
	return "chatbox_inbox_availability"
}

func (sess *InboxAvailability) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// DayAvailability 每天的可用时段
type DayAvailability struct {
	DayOfWeek int        `bson:"day_of_week" json:"day_of_week"` // 0=Sunday … 6=Saturday
	Enabled   bool       `bson:"enabled"     json:"enabled"`     // 是否启用这一天
	AllDay    bool       `bson:"all_day"     json:"all_day"`     // 是否全天可用
	Slots     []TimeSlot `bson:"slots"       json:"slots"`       // 多个时间段（如 09:00-12:00, 13:00-17:00）
}

type TimeSlot struct {
	Start string `bson:"start" json:"start"` // "09:00"
	End   string `bson:"end"   json:"end"`   // "17:00"
}

//
//// CSATSetting
//{
//"tenant_id": "tenant001",
//"account_id": "135774",
//"inbox_id": "inbox001",
//"enabled": true,
//"display": "star",
//"message": "请给我们本次服务打分，谢谢您的反馈！",
//"rule_type": "include",
//"rule_tags": ["售后", "VIP"],
//"updated_at": "2025-09-20T16:00:00Z"
//}
//
//// CSATResponse
//{
//"tenant_id": "tenant001",
//"account_id": "135774",
//"inbox_id": "inbox001",
//"conversation_id": "conv123",
//"agent_id": "客服1",
//"rating": 5,
//"comment": "服务非常好！",
//"created_at": "2025-09-20T16:10:00Z"
//}

// InboxCSATSetting 收件箱的 CSAT 问卷配置
type InboxCSATSetting struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"     json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"         json:"tenant_id"`
	AccountID string             `bson:"account_id"        json:"account_id"`
	InboxID   string             `bson:"inbox_id"          json:"inbox_id"`

	Enabled bool   `bson:"enabled"          json:"enabled"`           // 是否启用
	Display string `bson:"display"          json:"display"`           // "emoji" | "star"
	Message string `bson:"message"          json:"message,omitempty"` // 提示消息

	RuleType string   `bson:"rule_type"      json:"rule_type"`           // "include" | "exclude"
	RuleTags []string `bson:"rule_tags"      json:"rule_tags,omitempty"` // 触发规则的标签ID

	UpdatedAt time.Time `bson:"updated_at"    json:"updated_at"`
}

func (sess *InboxCSATSetting) GetTableName() string {
	return "chatbox_inbox_cast_setting"
}

func (sess *InboxCSATSetting) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// CSATResponse 用户提交的满意度反馈
type CSATResponse struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"  json:"_id,omitempty"`
	TenantID       string             `bson:"tenant_id"      json:"tenant_id"`
	AccountID      string             `bson:"account_id"     json:"account_id"`
	InboxID        string             `bson:"inbox_id"       json:"inbox_id"`
	ConversationID string             `bson:"conversation_id" json:"conversation_id"`
	AgentID        string             `bson:"agent_id"       json:"agent_id"`

	Rating  int    `bson:"rating"       json:"rating"` // 1-5（星级）或 -2~-2（表情映射）
	Comment string `bson:"comment"      json:"comment,omitempty"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

func (sess *CSATResponse) GetTableName() string {
	return "chatbox_inbox_cast_resp"
}

func (sess *CSATResponse) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// InboxPreChatForm {
// "tenant_id": "tenant001",
// "account_id": "135774",
// "inbox_id": "inbox001",
// "enabled": true,
// "message": "Share your queries or comments here.",
// "fields": [
// {
// "key": "emailAddress",
// "type": "email",
// "required": true,
// "label": "Email Id",
// "placeholder": "emailAddress"
// },
// {
// "key": "fullName",
// "type": "text",
// "required": false,
// "label": "Full name",
// "placeholder": "fullName"
// },
// {
// "key": "phoneNumber",
// "type": "text",
// "required": false,
// "label": "Phone number",
// "placeholder": "phoneNumber"
// }
// ],
// "updated_at": "2025-09-20T16:30:00Z"
// }
// InboxPreChatForm 收件箱的预聊天表单配置
type InboxPreChatForm struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"     json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"         json:"tenant_id"`
	AccountID string             `bson:"account_id"        json:"account_id"`
	InboxID   string             `bson:"inbox_id"          json:"inbox_id"`

	Enabled bool   `bson:"enabled"   json:"enabled"`           // 是否启用
	Message string `bson:"message"   json:"message,omitempty"` // 预聊天提示消息

	Fields []PreChatField `bson:"fields" json:"fields"` // 字段配置列表

	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// PreChatField 字段定义
type PreChatField struct {
	Key         string `bson:"key"        json:"key"`      // emailAddress / fullName / phoneNumber
	Type        string `bson:"type"       json:"type"`     // email / text / phone / select
	Required    bool   `bson:"required"   json:"required"` // 是否必填
	Label       string `bson:"label"      json:"label"`    // 显示标签
	Placeholder string `bson:"placeholder" json:"placeholder"`
}
