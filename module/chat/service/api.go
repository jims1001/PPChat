package service

import (
	"PProject/global"
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
	ctx := context.Background()
	defer ctx.Done()

	//TODO tenant_001 后面再处理
	result, _, _, err := ListMessages(ctx, "tenant_001", params.ConversationID, 0, params.Limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
	c.JSON(http.StatusOK, result)
}
