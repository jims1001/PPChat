package service

import (
	"PProject/global"
	"PProject/logger"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ListMessagesReq struct {
	ConversationID string `form:"conversationId" binding:"required"`
	LastSeq        int64  `form:"lastSeq,default=0"`
	Limit          int64  `form:"limit,default=50"`
}

func HandlerListMessages(c *gin.Context) {
	var params ListMessagesReq
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if params.Limit > 200 {
		params.Limit = 200
	}
	_, err := global.GetAuthInfo(c)

	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	defer ctx.Done()

	//TODO tenant_001 后面再处理
	result, _, _, err := ListMessages(ctx, "tenant_001", params.ConversationID, 0, params.Limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
	c.JSON(http.StatusOK, result)
}

// ListConversationsRequest 请求体结构，可扩展分页、筛选条件
type ListConversationsRequest struct {
	Page  int `json:"page,omitempty"`  // 页码，可选
	Limit int `json:"limit,omitempty"` // 每页数量，可选
}

// HandlerListConversations
// POST 获取当前用户的所有会话列表（带对方用户信息）
func HandlerListConversations(c *gin.Context) {
	var req ListConversationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	auth, err := global.GetAuthInfo(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	ctx := context.Background()
	defer ctx.Done()

	// 查询用户所有会话
	convList, err := FindUserConversations(ctx, "tenant_001", auth.UserId)
	if err != nil {
		logger.Errorf("FindUserConversations failed: %v user_id:%v", err, auth.UserId)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, convList)
}
