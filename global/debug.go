package global

import (
	"fmt"
	"github.com/gin-gonic/gin"
)

func DebugBody(c *gin.Context) {
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.String(500, "read body error: %v", err)
		return
	}
	fmt.Println("body string:", string(bodyBytes))
}
