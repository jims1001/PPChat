package model

import (
	"PProject/service/mgo"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Agent [
// {
// "name": "客服",
// "code": "agent",
// "desc": "普通客服，可以处理客户对话"
// },
// {
// "name": "管理员",
// "code": "admin",
// "desc": "系统管理员，拥有全部权限"
// },
// {
// "name": "主管",
// "code": "manager",
// "desc": "团队主管，可以分配会话、查看报表"
// }
// ]
// Agent 客服坐席

type Agent struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"       json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"           json:"tenant_id"`
	AccountID string             `bson:"account_id"          json:"account_id"`

	Name   string `bson:"name"                json:"name"`   // 客服名称
	Role   string `bson:"role"                json:"role"`   // "agent" | "manager" | "admin"
	Email  string `bson:"email"               json:"email"`  // 邮箱（登录用）
	Status int    `bson:"status"              json:"status"` // 1=正常, 0=禁用

	TeamIDs []string `bson:"team_ids,omitempty"  json:"team_ids,omitempty"` // 所属团队 ID 列表

	CreatedAt time.Time `bson:"created_at"          json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"          json:"updated_at"`
}

func (sess *Agent) GetTableName() string {
	return "chatbox_agent"
}

func (sess *Agent) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// Role 角色表
type Role struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"     json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"         json:"tenant_id"` // 多租户隔离
	AccountID string             `bson:"account_id"        json:"account_id"`

	Name string `bson:"name"              json:"name"`           // 角色名称，如：客服、管理员、主管
	Code string `bson:"code"              json:"code"`           // 唯一编码，如 "agent", "admin", "manager"
	Desc string `bson:"desc,omitempty"    json:"desc,omitempty"` // 描述

	BuiltIn bool `bson:"built_in"          json:"built_in"` // 是否系统内置角色

	// —— 验证/状态 —— //
	Status        string     `bson:"status"               json:"status"` // pending|active|disabled
	EmailVerified bool       `bson:"email_verified"       json:"email_verified"`
	VerifiedAt    *time.Time `bson:"verified_at,omitempty" json:"verified_at,omitempty"`

	// —— 认证（可选，本地账户时使用；SSO可不填） —— //
	AuthProvider string `bson:"auth_provider,omitempty" json:"auth_provider,omitempty"`   // local|sso|ldap|oauth
	PasswordHash string `bson:"password_hash,omitempty" json:"password_hash,omitempty"`   // bcrypt
	TwoFAEnabled bool   `bson:"two_fa_enabled,omitempty" json:"two_fa_enabled,omitempty"` // 开关

	CreatedAt time.Time `bson:"created_at"        json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"        json:"updated_at"`
}

func (sess *Role) GetTableName() string {
	return "chatbox_agent_role"
}

func (sess *Role) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// AgentVerification AgentInvite/Verification  邀请与邮箱验证记录（一次性、带过期）
type AgentVerification struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"       json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"           json:"tenant_id"`
	AccountID string             `bson:"account_id"          json:"account_id"`
	AgentID   string             `bson:"agent_id,omitempty"  json:"agent_id,omitempty"` // 邀请时可能还没有Agent，留空也可
	Email     string             `bson:"email"               json:"email"`

	Purpose   string     `bson:"purpose"             json:"purpose"`    // invite | email_verify | reset_password
	TokenHash string     `bson:"token_hash"          json:"token_hash"` // SHA256(token)
	ExpiresAt time.Time  `bson:"expires_at"          json:"expires_at"`
	UsedAt    *time.Time `bson:"used_at,omitempty"   json:"used_at,omitempty"`

	CreatedBy string    `bson:"created_by"          json:"created_by"` // 谁发出的邀请
	CreatedAt time.Time `bson:"created_at"          json:"created_at"`
}

func (sess *AgentVerification) GetTableName() string {
	return "chatbox_agent_verification`"
}

func (sess *AgentVerification) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// Permission 权限表（系统里的功能点）
type Permission struct {
	ID   primitive.ObjectID `bson:"_id,omitempty"  json:"_id,omitempty"`
	Code string             `bson:"code"           json:"code"` // 比如: conversation.view, conversation.close
	Name string             `bson:"name"           json:"name"`
	Desc string             `bson:"desc,omitempty" json:"desc,omitempty"`
}

func (sess *Permission) GetTableName() string {
	return "chatbox_agent_permission"
}

func (sess *Permission) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// RolePermission 角色与权限的绑定
type RolePermission struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"   json:"_id,omitempty"`
	RoleID       string             `bson:"role_id"        json:"role_id"`
	PermissionID string             `bson:"permission_id"  json:"permission_id"`
}

func (sess *RolePermission) GetTableName() string {
	return "chatbox_agent_role_permission"
}

func (sess *RolePermission) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// ValidateCreateAgentDTO
//校验：同租户下 email 不重复；role_id 合法；name 非空
//
//新建 Agent{Status: "pending", EmailVerified:false}
//
//生成一次性 token（32~48字节随机），存 AgentVerification（purpose="invite"，expires_at=48h，token_hash=SHA256(token)）
//
//发送邀请邮件：链接类似
//
//https://yourhost/agents/accept?token=<rawToken>
//
//
//返回成功
//
//B. 被邀请人点击链接 → 接口 AcceptInvite(token)
//
//查 AgentVerification：token 哈希匹配、purpose=invite、未过期、未使用
//
//（可选）要求设置密码/开启 2FA
//
//标记 used_at，更新 Agent{Status:"active", EmailVerified:true, VerifiedAt:now}
//
//记录一条审计日志（ConversationLog/OpLog 不赘述）
//
//C. 重发邀请
//
//关闭上一条未用的验证记录（或直接再插一条新记录，旧的在 TTL 后自动过期）；发送新邮件。

type AgentConversation struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	TenantID       string             `bson:"tenant_id"     json:"tenant_id"`
	ConversationID string             `bson:"conversation_id" json:"conversation_id"`
	AgentID        string             `bson:"agent_id"      json:"agent_id"`
	ContactID      string             `bson:"contact_id"    json:"contact_id"`                // 联系人ID
	InboxID        string             `bson:"inbox_id"      json:"inbox_id"`                  // 收件箱ID
	TeamID         string             `bson:"team_id,omitempty"     json:"team_id,omitempty"` // 已分配的团队
	Priority       string             `bson:"priority"              json:"priority"`          // none|low|normal|high|urgent
	TagIDs         []string           `bson:"tag_ids,omitempty"     json:"tag_ids,omitempty"` // 对话标签

	Role     string     `bson:"role"     json:"role"`   // "assignee" | "collaborator" | "observer"
	Status   string     `bson:"status"   json:"status"` // active/inactive
	JoinedAt time.Time  `bson:"joined_at" json:"joined_at"`
	LeftAt   *time.Time `bson:"left_at,omitempty" json:"left_at,omitempty"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

func (sess *AgentConversation) GetTableName() string {
	return "chatbox_agent_conversation"
}

func (sess *AgentConversation) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

//+------------------+          +----------------------+          +------------------------+
//|  Conversation    | 1      * |  AgentConversation   | *      1 |        Agent           |
//|------------------|----------|----------------------|----------|------------------------|
//| id               |          | id                   |          | id                     |
//| tenant_id        |          | conversation_id      |          | tenant_id              |
//| assignee_id (*)  |          | agent_id             |          | name                   |
//| subject          |          | role (assignee)      |          | email                  |
//| status           |          | status               |          | role                   |
//+------------------+          +----------------------+          +------------------------+
//|
//| 1
//|
//|      *
//+------------------------+
//|  AgentCollaborator     |
//|------------------------|
//| id                     |
//| tenant_id              |
//| conversation_id        |
//| agent_id               |
//| role (collaborator)    |
//| status (active/inact)  |
//| joined_at              |
//| left_at                |
//+------------------------+

// 协作者关联表
type AgentCollaborator struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	TenantID       string             `bson:"tenant_id" json:"tenant_id"`
	ConversationID string             `bson:"conversation_id" json:"conversation_id"` // 聊天ID
	AgentID        string             `bson:"agent_id" json:"agent_id"`               // 代理ID

	Role     string     `bson:"role" json:"role"`     // collaborator 固定值；也可以支持 observer
	Status   string     `bson:"status" json:"status"` // active | inactive
	JoinedAt time.Time  `bson:"joined_at" json:"joined_at"`
	LeftAt   *time.Time `bson:"left_at,omitempty" json:"left_at,omitempty"`
}

func (sess *AgentCollaborator) GetTableName() string {
	return "chatbox_agent_conversation"
}

func (sess *AgentCollaborator) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// AgentContact 客服联系人
type AgentContact struct {
	ID       primitive.ObjectID `bson:"_id,omitempty"         json:"id,omitempty"`
	TenantID string             `bson:"tenant_id"             json:"tenant_id"` // 租户ID，区分不同租户
	Name     string             `bson:"name"                  json:"name"`      // 联系人姓名
	Email    string             `bson:"email,omitempty"       json:"email,omitempty"`
	Phone    string             `bson:"phone,omitempty"       json:"phone,omitempty"`
	Company  string             `bson:"company,omitempty"     json:"company,omitempty"`
	Country  string             `bson:"country,omitempty"     json:"country,omitempty"`
	City     string             `bson:"city,omitempty"        json:"city,omitempty"`

	FacebookURL string `bson:"facebook_url,omitempty" json:"facebook_url,omitempty"`
	TwitterURL  string `bson:"twitter_url,omitempty"  json:"twitter_url,omitempty"`
	LinkedinURL string `bson:"linkedin_url,omitempty" json:"linkedin_url,omitempty"`
	GithubURL   string `bson:"github_url,omitempty"   json:"github_url,omitempty"`

	Notes string `bson:"notes,omitempty"        json:"notes,omitempty"` // 简介/备注

	CreatedAt time.Time `bson:"created_at,omitempty"   json:"created_at,omitempty"`
	UpdatedAt time.Time `bson:"updated_at,omitempty"   json:"updated_at,omitempty"`
}

func (sess *AgentContact) GetTableName() string {
	return "chatbox_agent_contact"
}

func (sess *AgentContact) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

type AgentConversationMeta struct {
	StartedAt   time.Time `bson:"started_at,omitempty"    json:"started_at,omitempty"`
	Browser     string    `bson:"browser,omitempty"       json:"browser,omitempty"`
	BrowserLang string    `bson:"browser_lang,omitempty"  json:"browser_lang,omitempty"`
	OS          string    `bson:"os,omitempty"            json:"os,omitempty"`
	IPAddress   string    `bson:"ip_address,omitempty"    json:"ip_address,omitempty"`
	Country     string    `bson:"country,omitempty"       json:"country,omitempty"`
	City        string    `bson:"city,omitempty"          json:"city,omitempty"`
	Referrer    string    `bson:"referrer,omitempty"      json:"referrer,omitempty"`
	UserAgent   string    `bson:"user_agent,omitempty"    json:"user_agent,omitempty"`
}

func (sess *AgentConversationMeta) GetTableName() string {
	return "chatbox_agent_conversation_meta"
}

func (sess *AgentConversationMeta) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// AgentBot {
// "_id": { "$oid": "650c2f82d91c34a9e8b4d111" },
// "tenant_id": "tenant_001",
// "name": "FAQ机器人",
// "avatar_url": "https://example.com/bot-avatar.png",
// "description": "用于自动回答常见问题的机器人",
// "webhook_url": "https://example.com/webhook",
// "status": "enabled",
// "created_at": { "$date": "2025-09-21T02:30:00Z" },
// "updated_at": { "$date": "2025-09-21T02:30:00Z" }
// }
type AgentBot struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"    json:"id,omitempty"`
	TenantID    string             `bson:"tenant_id"        json:"tenant_id"`                  // 租户ID
	Name        string             `bson:"name"             json:"name"`                       // 机器人名称
	AvatarURL   string             `bson:"avatar_url,omitempty" json:"avatar_url,omitempty"`   // 机器人头像
	Description string             `bson:"description,omitempty" json:"description,omitempty"` // 描述
	WebhookURL  string             `bson:"webhook_url,omitempty" json:"webhook_url,omitempty"` // 回调 Webhook 地址

	Status    string    `bson:"status"           json:"status"` // 启用/禁用
	CreatedAt time.Time `bson:"created_at"       json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"       json:"updated_at"`
}

func (sess *AgentBot) GetTableName() string {
	return "chatbox_agent_bot"
}

func (sess *AgentBot) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}
