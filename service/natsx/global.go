package natsx

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

var (
	globalMgr *NatsManager
	startOnce sync.Once

	mu               sync.Mutex
	pendingRoutes    = make(map[string]NatsxRoute)     // 启动前缓存的路由
	pendingHandlers  = make(map[string][]NatsxHandler) // 启动前缓存的订阅回调
	registeredBizSet = make(map[string]struct{})       // 已注册的 Biz（幂等）
	subscribedBizSet = make(map[string]struct{})       // 已订阅的 Biz（幂等）
	defaultMws       []NatsxMiddleware                 // 全局中间件
)

// UseGlobalMiddlewares 启动前配置全局中间件（例如幂等）
func UseGlobalMiddlewares(mws ...NatsxMiddleware) {
	mu.Lock()
	defer mu.Unlock()
	defaultMws = append(defaultMws, mws...)
}

// StartNats 启动全局 NATS（只会执行一次）。
// 会把启动前通过 RegisterRoute / RegisterHandler 缓存的内容一次性应用。
func StartNats(cfg ...NatsxConfig) {
	startOnce.Do(func() {
		var c NatsxConfig
		if len(cfg) > 0 {
			c = cfg[0]
		} else {
			c = NatsxConfig{
				Servers: []string{"nats://127.0.0.1:4222"},
				Name:    "global-nats",
			}
		}

		mu.Lock()
		mws := append([]NatsxMiddleware(nil), defaultMws...)
		mu.Unlock()

		mgr, err := NewNatsManager(c, mws...)
		if err != nil {
			log.Fatalf("failed to start NatsManager: %v", err)
		}
		globalMgr = mgr

		// 在独立 goroutine 里应用启动前缓存的路由与订阅
		go func() {
			mu.Lock()
			defer mu.Unlock()

			// 1) 先注册所有路由
			for biz, r := range pendingRoutes {
				if err := globalMgr.RegisterRoute(r); err != nil {
					log.Printf("register route failed (biz=%s): %v", biz, err)
					continue
				}
				registeredBizSet[biz] = struct{}{}
			}

			// 2) 再订阅所有 handler
			for biz, hs := range pendingHandlers {
				for _, h := range hs {
					if err := globalMgr.Subscribe(biz, h); err != nil {
						log.Printf("subscribe failed (biz=%s): %v", biz, err)
						continue
					}
				}
				subscribedBizSet[biz] = struct{}{}
			}

			// 清理缓存
			pendingRoutes = make(map[string]NatsxRoute)
			pendingHandlers = make(map[string][]NatsxHandler)
			log.Println("NatsManager started and applied pending routes/handlers.")
		}()
	})
}

// StopNats 优雅关闭（可选）
func StopNats() error {
	mu.Lock()
	defer mu.Unlock()
	if globalMgr == nil {
		return nil
	}
	err := globalMgr.Close()
	globalMgr = nil
	return err
}

// GetNatsManager 获取全局单例（未启动时返回错误）
func GetNatsManager() (*NatsManager, error) {
	if globalMgr == nil {
		return nil, errors.New("NatsManager not started: call StartNats() first")
	}
	return globalMgr, nil
}

// ---------- 对外暴露的全局操作（可在启动前/后调用） ----------

// RegisterRoute 全局注册路由（对外暴露）
func RegisterRoute(r NatsxRoute) error {
	mu.Lock()
	defer mu.Unlock()

	// 幂等：同 Biz 重复注册直接跳过（你也可以改成返回错误）
	if _, ok := registeredBizSet[r.Biz]; ok {
		return nil
	}

	if globalMgr == nil {
		// 启动前：先缓存
		pendingRoutes[r.Biz] = r
		registeredBizSet[r.Biz] = struct{}{}
		return nil
	}

	// 启动后：直接注册到运行中的实例
	if err := globalMgr.RegisterRoute(r); err != nil {
		return err
	}
	registeredBizSet[r.Biz] = struct{}{}
	return nil
}

// RegisterHandler 为某个 Biz 注册订阅处理器（对外暴露）
func RegisterHandler(biz string, h NatsxHandler) error {
	mu.Lock()
	defer mu.Unlock()

	if globalMgr == nil {
		// 启动前：先缓存
		pendingHandlers[biz] = append(pendingHandlers[biz], h)
		return nil
	}

	// 启动后：立即订阅
	if err := globalMgr.Subscribe(biz, h); err != nil {
		return err
	}
	subscribedBizSet[biz] = struct{}{}
	return nil
}

// Publish 对外发布消息（需要已启动）
func Publish(ctx context.Context, biz string, data []byte, hdr map[string]string) error {
	mu.Lock()
	m := globalMgr
	mu.Unlock()
	if m == nil {
		return errors.New("NatsManager not started")
	}
	return m.Publish(ctx, biz, data, hdr)
}

// PublishOnce 对外发布消息（带 Nats-Msg-Id 去重）
func PublishOnce(ctx context.Context, biz string, data []byte, hdr map[string]string, msgID string) error {
	mu.Lock()
	m := globalMgr
	mu.Unlock()
	if m == nil {
		return errors.New("NatsManager not started")
	}
	return m.PublishOnce(ctx, biz, data, hdr, msgID)
}

// PullConsume 对外拉批消费（JetStream Pull）
// 可在任意时刻调用；若路由未注册会失败
func PullConsume(ctx context.Context, biz string, batch int, wait time.Duration, h NatsxHandler) error {
	mu.Lock()
	m := globalMgr
	mu.Unlock()
	if m == nil {
		return errors.New("NatsManager not started")
	}
	return m.PullConsume(ctx, biz, batch, wait, h)
}
