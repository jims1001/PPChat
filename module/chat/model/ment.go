package model

import (
	"PProject/service/mgo"

	"go.mongodb.org/mongo-driver/mongo"
)

// MentionIndex collection field constants
const (
	MIFieldTenantID       = "tenant_id"
	MIFieldConversationID = "conversation_id"
	MIFieldTargetUserID   = "target_user_id"
	MIFieldSeq            = "seq"
	MIFieldKind           = "kind"
	MIFieldCreatedAt      = "created_at"
)

// MentionIndex 存储@某人的 游标
type MentionIndex struct {
	TenantID       string `bson:"tenant_id"`
	ConversationID string `bson:"conversation_id"`
	TargetUserID   string `bson:"target_user_id"`
	Seq            int64  `bson:"seq"`
	Kind           string `bson:"kind"` // "mention"|"reply"
	CreatedAt      int64  `bson:"created_at"`
}

func (sess *MentionIndex) GetTableName() string {
	return "mention_index"
}

func (sess *MentionIndex) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}
