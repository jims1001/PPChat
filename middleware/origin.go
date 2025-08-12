package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Origin validation example: modify according to your own domain/Token logic
func Origin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet && c.Request.URL.Path == "/ws" {
			// Example: you can validate Header/Cookie/JWT, etc.
			// token := c.GetHeader("X-Token")
			// if token == "" { c.AbortWithStatus(401); return }
		}
		c.Next()
	}
}
