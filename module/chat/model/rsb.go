package model

import (
	"PProject/service/mgo"

	"go.mongodb.org/mongo-driver/mongo"
)

// ReadSparseBlock collection field constants
const (
	RSBFieldTenantID       = "tenant_id"
	RSBFieldConversationID = "conversation_id"
	RSBFieldUserID         = "user_id"
	RSBFieldBlockStart     = "block_start"
	RSBFieldBits           = "bits"
	RSBFieldUpdatedAt      = "updated_at"
)

// ReadSparseBlock 存放读取了哪些信息
type ReadSparseBlock struct {
	TenantID       string `bson:"tenant_id"`
	ConversationID string `bson:"conversation_id"`
	UserID         string `bson:"user_id"`
	BlockStart     int64  `bson:"block_start"`
	Bits           []byte `bson:"bits"` // len = BlockK/8
	UpdatedAt      int64  `bson:"updated_at"`
}

func (sess *ReadSparseBlock) GetTableName() string {
	return "read_sparse_block"
}

func (sess *ReadSparseBlock) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}
