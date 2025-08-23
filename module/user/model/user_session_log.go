package model

type UserSessionLog struct {
	LogId string `bson:"session_log_id" json:"session_log_id"` // 会话ID（UUID/雪花）
	UserSession
}
