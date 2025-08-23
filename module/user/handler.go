package user

import (
	service "PProject/module/user/service"
	"PProject/tools/ids"
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
)

func HandlerLogin(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"token": "abc123",
		"user": gin.H{
			"id":   1001,
			"name": "Alice",
		},
	})

	var loginParams service.LoginParams
	if err := c.ShouldBindJSON(&loginParams); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	sessionId := ids.GenerateString()
	loginParams.SessionID = sessionId

	ctx := context.Background()
	defer ctx.Done()

}
