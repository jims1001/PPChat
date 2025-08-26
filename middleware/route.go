package middleware

import (
	midsec "PProject/middleware/security"
	"github.com/gin-gonic/gin"
)

// 配置选项
type RouteOpt struct {
	IsAuth bool
}

// 封装 POST
func POST(r gin.IRoutes, path string, handler gin.HandlerFunc, opt RouteOpt) {
	if opt.IsAuth {
		r.POST(path,
			// 这里替换成你实际的 Auth 中间件
			midsec.Middleware(midsec.DefaultOptions()),
			handler,
		)
	} else {
		r.POST(path, handler)
	}
}

// 封装 GET
func GET(r gin.IRoutes, path string, handler gin.HandlerFunc, opt RouteOpt) {
	if opt.IsAuth {
		r.GET(path,
			midsec.Middleware(midsec.DefaultOptions()),
			handler,
		)
	} else {
		r.GET(path, handler)
	}
}
