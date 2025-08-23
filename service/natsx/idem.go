package natsx

import (
	"context"
	"strings"
	"sync"
	"time"
)

// ----- 抽象存储 -----
type IdemStore interface {
	SeenOnce(key string, ttl time.Duration) (seen bool, err error)
}

// ----- 内存实现（单进程） -----
type memIdem struct {
	mu  sync.Mutex
	m   map[string]int64 // key -> expireUnix
	ttl time.Duration
}

func NewMemIdem(defaultTTL time.Duration) IdemStore {
	mi := &memIdem{m: make(map[string]int64), ttl: defaultTTL}
	// 清理协程
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for range t.C {
			now := time.Now().Unix()
			mi.mu.Lock()
			for k, exp := range mi.m {
				if exp <= now {
					delete(mi.m, k)
				}
			}
			mi.mu.Unlock()
		}
	}()
	return mi
}

func (mi *memIdem) SeenOnce(key string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = mi.ttl
	}
	exp := time.Now().Add(ttl).Unix()
	mi.mu.Lock()
	defer mi.mu.Unlock()
	if old, ok := mi.m[key]; ok && old > time.Now().Unix() {
		return true, nil // 已见过
	}
	mi.m[key] = exp
	return false, nil
}

// ----- 从消息头提取 msgID -----
func msgIDFromHeader(h map[string]string) string {
	// 标准头：Nats-Msg-Id；你也可以用业务自定义如 X-Msg-Id
	for _, k := range []string{"Nats-Msg-Id", "nats-msg-id", "X-Msg-Id", "x-msg-id"} {
		if v, ok := h[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

// ----- 幂等中间件 -----
// 用法：NewNatsxConsumer(client, NatsxIdemMiddleware(store, ttl))
func NatsxIdemMiddleware(store IdemStore, ttl time.Duration) NatsxMiddleware {
	return func(next NatsxHandler) NatsxHandler {
		return func(ctx context.Context, msg NatsxMessage) error {
			id := msgIDFromHeader(msg.Header)
			if id == "" {
				// 无ID时根据 subject+内容构造一个弱ID（谨慎使用）
				id = msg.Subject + "|" + strings.TrimSpace(string(msg.Data))
			}
			seen, _ := store.SeenOnce(id, ttl)
			if seen {
				// 已处理过，直接跳过（记录日志可选）
				return nil
			}
			return next(ctx, msg)
		}
	}
}
