package user

import (
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
}
