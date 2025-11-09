package security

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// —— 跨域配置结构 —— //
type CORSOptions struct {
	AllowOrigins     []string // 允许的域名列表
	AllowMethods     []string // 允许的 HTTP 方法
	AllowHeaders     []string // 允许的请求头
	AllowCredentials bool     // 是否允许携带 Cookie
}

// —— 默认配置 —— //
func DefaultCORSOptions() *CORSOptions {
	return &CORSOptions{
		AllowOrigins:     []string{"https://localhost:5173", "https://admin.example.com"}, // ✅ 这里改成你要允许的域名
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "AuthorizationHash"},
		AllowCredentials: true,
	}
}

// —— 跨域中间件 —— //
func CORSMiddleware(opts *CORSOptions) gin.HandlerFunc {
	if opts == nil {
		opts = DefaultCORSOptions()
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			c.Next()
			return
		}

		// 判断是否在允许列表中
		allowOrigin := ""
		for _, o := range opts.AllowOrigins {
			if o == "*" {
				allowOrigin = "*"
				break
			}
			if strings.EqualFold(o, origin) {
				allowOrigin = origin
				break
			}
		}

		// 若不在允许列表内，不添加跨域头
		if allowOrigin == "" {
			c.Next()
			return
		}

		// —— 设置跨域头 —— //
		c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		c.Writer.Header().Set("Access-Control-Allow-Methods", strings.Join(opts.AllowMethods, ", "))
		c.Writer.Header().Set("Access-Control-Allow-Headers", strings.Join(opts.AllowHeaders, ", "))

		if opts.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// 处理预检请求
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
