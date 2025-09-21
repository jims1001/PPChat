package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CustomAttribute // 定义属性
// {
// "tenant_id": "tenant001",
// "account_id": "135774",
// "scope": "conversation",
// "display_name": "客户等级",
// "key": "customerLevel",
// "desc": "标记客户的重要程度",
// "type": "list",
// "options": ["普通", "VIP", "黑名单"],
// "required": false,
// "created_at": "2025-09-20T18:30:00Z",
// "updated_at": "2025-09-20T18:30:00Z"
// }
//
// // 存储属性值
// {
// "tenant_id": "tenant001",
// "account_id": "135774",
// "scope": "conversation",
// "ref_id": "conv123",
// "key": "customerLevel",
// "value": "VIP",
// "created_at": "2025-09-20T18:35:00Z",
// "updated_at": "2025-09-20T18:35:00Z"
// }
// CustomAttribute 定义属性
type CustomAttribute struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"   json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"       json:"tenant_id"`
	AccountID string             `bson:"account_id"      json:"account_id"`

	// 作用范围: conversation / contact
	Scope string `bson:"scope" json:"scope"`

	DisplayName string `bson:"display_name" json:"display_name"` // 展示名
	Key         string `bson:"key"          json:"key"`          // 唯一键
	Desc        string `bson:"desc"         json:"desc"`         // 描述信息

	Type    string   `bson:"type"         json:"type"`              // text / number / link / date / list / checkbox
	Options []string `bson:"options"      json:"options,omitempty"` // 列表/复选框的值

	Required bool `bson:"required" json:"required"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// CustomAttributeValue 保存会话或联系人的属性值
type CustomAttributeValue struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"   json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"       json:"tenant_id"`
	AccountID string             `bson:"account_id"      json:"account_id"`

	Scope string `bson:"scope" json:"scope"`   // conversation / contact
	RefID string `bson:"ref_id" json:"ref_id"` // 会话ID or 联系人ID
	Key   string `bson:"key"    json:"key"`    // 对应 CustomAttribute.Key
	Value string `bson:"value"  json:"value"`  // 值（JSON 字符串，可存数组）

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}
