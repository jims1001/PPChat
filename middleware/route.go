package middleware

import (
	midsec "PProject/middleware/security"

	"github.com/gin-gonic/gin"
)

// 配置选项
type RouteOpt struct {
	IsAuth bool
}

// 内部统一处理（避免重复代码）
func withOpt(r gin.IRoutes, opt RouteOpt, handler gin.HandlerFunc) []gin.HandlerFunc {
	if opt.IsAuth {
		return []gin.HandlerFunc{
			midsec.Middleware(midsec.DefaultOptions()),
			handler,
		}
	}
	return []gin.HandlerFunc{handler}
}

// POST
func POST(r gin.IRoutes, path string, handler gin.HandlerFunc, opt RouteOpt) {
	r.POST(path, withOpt(r, opt, handler)...)
}

// GET
func GET(r gin.IRoutes, path string, handler gin.HandlerFunc, opt RouteOpt) {
	r.GET(path, withOpt(r, opt, handler)...)
}

// PATCH
func PATCH(r gin.IRoutes, path string, handler gin.HandlerFunc, opt RouteOpt) {
	r.PATCH(path, withOpt(r, opt, handler)...)
}

// PUT
func PUT(r gin.IRoutes, path string, handler gin.HandlerFunc, opt RouteOpt) {
	r.PUT(path, withOpt(r, opt, handler)...)
}

// DELETE
func DELETE(r gin.IRoutes, path string, handler gin.HandlerFunc, opt RouteOpt) {
	r.DELETE(path, withOpt(r, opt, handler)...)
}
