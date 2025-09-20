package model

import (
	"PProject/service/mgo"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Tag db.tags.find({ tenant_id: "xxx", account_id: "135774" })
type Tag struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"      json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"          json:"tenant_id"`
	AccountID string             `bson:"account_id"         json:"account_id"`

	Name  string `bson:"name"               json:"name"`            // 标签名称
	Color string `bson:"color,omitempty"    json:"color,omitempty"` // 可选：UI 渲染用，比如 #FF0000
	Type  string `bson:"type,omitempty"     json:"type,omitempty"`  // 可选：区分是对话标签 / 消息标签

	CreatedBy string    `bson:"created_by,omitempty" json:"created_by,omitempty"` // 谁创建的
	CreatedAt time.Time `bson:"created_at"         json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"         json:"updated_at"`
}

func (sess *Tag) GetTableName() string {
	return "tag"
}

func (sess *Tag) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}
