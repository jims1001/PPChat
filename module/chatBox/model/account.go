package model

import (
	"PProject/service/mgo"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

//{
//"tenant_id": "tenant_001",
//"account_id": "135774",
//"scope_type": "account",
//"enabled": true,
//"inactive_threshold_seconds": 900,
//"inactive_raw_value": 15,
//"inactive_raw_unit": "minute",
//"resolve_message": "由于闲置 15 分钟，对话已被系统标记已解决",
//"skip_waiting_conversations": false,
//"post_resolve_tag_ids": ["tag_silent_close"],
//"created_at": "2025-09-20T13:00:00Z",
//"updated_at": "2025-09-20T13:00:00Z"
//}

// AccountSetting 账户设置
type AccountSetting struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"          json:"_id,omitempty"`
	TenantID     string             `bson:"tenant_id"              json:"tenant_id"`     // 多租户隔离
	AccountID    string             `bson:"account_id"             json:"account_id"`    // 系统生成的唯一账号 ID
	AccountName  string             `bson:"account_name"           json:"account_name"`  // 账户名称
	SiteLanguage string             `bson:"site_language"          json:"site_language"` // zh-CN / en-US
	AutoResolve  bool               `bson:"auto_resolve"           json:"auto_resolve"`  // 自动解决对话开关
	Status       int                `bson:"status"                 json:"status"`        // 1=正常, 0=已删除
	CreatedAt    time.Time          `bson:"created_at"             json:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at"             json:"updated_at"`
	DeletedAt    *time.Time         `bson:"deleted_at,omitempty"   json:"deleted_at,omitempty"`
}

func (sess *AccountSetting) GetTableName() string {
	return "chat_agent_account_settings"
}

func (sess *AccountSetting) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

type AutoResolvePolicy struct {
	ID primitive.ObjectID `bson:"_id,omitempty"          json:"_id,omitempty"`

	// 归属与作用范围
	TenantID  string `bson:"tenant_id"              json:"tenant_id"`
	AccountID string `bson:"account_id"             json:"account_id"`
	// 作用域：account / inbox / team；便于做覆盖策略（先查更细粒度，缺省回落到account）
	ScopeType string `bson:"scope_type"             json:"scope_type"` // "account" | "inbox" | "team"
	ScopeID   string `bson:"scope_id,omitempty"     json:"scope_id,omitempty"`

	// 总开关（页面右上角的 toggle）
	Enabled bool `bson:"enabled"                json:"enabled"`

	// —— 无活动持续时间 —— //
	// 为了计算方便存秒；同时保留原始值与单位，便于 UI 回显
	InactiveThresholdSeconds int64  `bson:"inactive_threshold_seconds" json:"inactive_threshold_seconds"` // 例如 15*60
	InactiveRawValue         int64  `bson:"inactive_raw_value"         json:"inactive_raw_value"`         // 例如 15
	InactiveRawUnit          string `bson:"inactive_raw_unit"          json:"inactive_raw_unit"`          // "minute" | "hour" | "day"

	// —— 自定义自动解决消息 —— //
	// 发送给客户的文案（富文本可换成 content+content_type）
	ResolveMessage string `bson:"resolve_message,omitempty"   json:"resolve_message,omitempty"`

	// —— 偏好设置 —— //
	// “跳过等待客服回复的会话”开关
	SkipWaitingConversations bool `bson:"skip_waiting_conversations" json:"skip_waiting_conversations"`
	// “自动解决后添加标签”：存标签ID；如只允许一个也用切片，便于将来扩展为多选
	PostResolveTagIDs []string `bson:"post_resolve_tag_ids,omitempty" json:"post_resolve_tag_ids,omitempty"`

	// 审计
	UpdatedBy string    `bson:"updated_by,omitempty"         json:"updated_by,omitempty"` // 操作人ID
	CreatedAt time.Time `bson:"created_at"                   json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"                   json:"updated_at"`
}

func (sess *AutoResolvePolicy) GetTableName() string {
	return "chat_agent_policy"
}

func (sess *AutoResolvePolicy) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

func find(ctx context.Context, tenantID, accountID, scopeType, scopeID string) *AutoResolvePolicy {
	filter := bson.M{
		"tenant_id":  tenantID,
		"account_id": accountID,
		"scope_type": scopeType,
	}
	if scopeID != "" {
		filter["scope_id"] = scopeID
	}

	var p AutoResolvePolicy
	err := p.Collection().FindOne(ctx, filter).Decode(&p)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil
		}
		// 生产环境建议记录日志或返回 error
		return nil
	}
	return &p
}

// 优先级：team > inbox > account
//func LoadPolicy(ctx, tenantID, accountID, inboxID, teamID string) (*AutoResolvePolicy, error) {
//	// 1. team 级
//	if p := find(tenantID, accountID, "team", teamID); p != nil {
//		return p, nil
//	}
//	// 2. inbox 级
//	if p := find(tenantID, accountID, "inbox", inboxID); p != nil {
//		return p, nil
//	}
//	// 3. account 级
//	return find(tenantID, accountID, "account", "")
//}
