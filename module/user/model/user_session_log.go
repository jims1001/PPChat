package model

import (
	"PProject/service/mgo"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserSessionLog struct {
	LogId string `bson:"session_log_id" json:"session_log_id"` // 会话ID（UUID/雪花）
	UserSession
}

func (log *UserSessionLog) GetTableName() string {
	return "user_session_log"
}

func (log *UserSessionLog) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(log.GetTableName())
}
