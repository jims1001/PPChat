package security

import (
	error "PProject/tools/errs"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

// —— context key ——
// 你后续模块可统一用这俩 key 读取
const (
	PPCtxAuthKey     = "authorization"     // string
	PPCtxAuthHashKey = "authorizationHash" // string
)

type Options struct {
	// 读取哪个请求头
	HeaderToken               string // 默认 "authorization"
	HeaderHash                string // 默认 "authorizationHash"
	EnableAuthorizationBearer bool   // 默认 true

	SetEmptyIntoContext bool // 默认 true
}

func DefaultOptions() *Options {
	return &Options{
		HeaderToken:               PPCtxAuthKey,
		HeaderHash:                PPCtxAuthHashKey,
		EnableAuthorizationBearer: true,
		SetEmptyIntoContext:       true,
	}
}

func Middleware(opts *Options) gin.HandlerFunc {
	if opts == nil {
		opts = DefaultOptions()
	}
	return func(c *gin.Context) {
		var token = strings.TrimSpace(c.GetHeader(opts.HeaderToken))
		var hash = strings.TrimSpace(c.GetHeader(opts.HeaderHash))

		// 兼容 Authorization: Bearer xxx
		if token == "" && opts.EnableAuthorizationBearer {
			if authz := strings.TrimSpace(c.GetHeader("Authorization")); authz != "" {
				if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
					token = strings.TrimSpace(authz[len("bearer "):])
				}
			}
		}

		// 写入 context（可选：即使为空也写，便于后续统一读取）
		if token != "" || (opts.SetEmptyIntoContext && token != "") {
			c.Set(PPCtxAuthKey, token)
		}
		if hash != "" || (opts.SetEmptyIntoContext && token != "") {
			c.Set(PPCtxAuthHashKey, hash)
		}

		if token == "" || hash == "" {
			c.AbortWithStatusJSON(http.StatusOK, error.ErrTokenExpired)
			return
		}

		c.Next()
	}
}
