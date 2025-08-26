package middleware

import (
	"sync"

	"github.com/gin-gonic/gin"
)

// 全局单例 + once
var (
	globalMgr *MiddlewareManager
	once      sync.Once
)

// MiddlewareManager 可以自由注册/注销中间件
type MiddlewareManager struct {
	mu   sync.RWMutex
	mids []gin.HandlerFunc
}

// Config ：在程序启动时显式初始化（可选）
func Config() {
	once.Do(func() {
		globalMgr = NewManager()
	})
}

// NewManager 创建新的实例
func NewManager() *MiddlewareManager {
	return &MiddlewareManager{}
}

// Manager ：获取全局实例（惰性初始化，线程安全）
func Manager() *MiddlewareManager {
	once.Do(func() {
		if globalMgr == nil {
			globalMgr = NewManager()
		}
	})
	return globalMgr // ✅ 注意不要写成 &globalMgr
}

// Add 注册一个中间件
func (m *MiddlewareManager) Add(h gin.HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mids = append(m.mids, h)
}

// Clear 清空全部中间件
func (m *MiddlewareManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mids = nil
}

// Use 返回一个 gin.HandlerFunc，作为总控挂载到 Engine 上
func (m *MiddlewareManager) Use() gin.HandlerFunc {
	return func(c *gin.Context) {
		m.mu.RLock()
		handlers := append([]gin.HandlerFunc{}, m.mids...) // 拷贝一份快照
		m.mu.RUnlock()

		for _, h := range handlers {
			h(c)
			if c.IsAborted() {
				return
			}
		}
		c.Next()
	}
}
