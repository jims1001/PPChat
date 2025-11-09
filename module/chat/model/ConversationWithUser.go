package model

import usermodel "PProject/module/user/model"

// ConversationWithUser 表示会话 + 对方用户信息
type ConversationWithUser struct {
	Conversation `bson:",inline"` // 内嵌原有会话字段
	UserInfo     usermodel.User   // 对方用户信息
}
