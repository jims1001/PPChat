package natsx

import (
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"strings"
	"sync"
	"time"
)

// NatsxMode 工作模式
type NatsxMode int

const (
	Core          NatsxMode = iota // 无持久化
	JetStreamPush                  // JS 推送订阅
	JetStreamPull                  // JS 拉取订阅
)

// NatsxRoute 路由配置（按 Biz 维度注册）
type NatsxRoute struct {
	Biz           string
	Subject       string
	Mode          NatsxMode
	Queue         string // 队列组（Core/JS Push）
	Durable       string // JS durable 名（建议设置）
	AckWait       time.Duration
	MaxAckPending int
}

// NatsxConfig 客户端配置
type NatsxConfig struct {
	Servers         []string
	Name            string
	ReconnectWait   time.Duration
	Timeout         time.Duration
	PublishAsyncMax int
}

// NatsxClient 统一客户端
type NatsxClient struct {
	cfg NatsxConfig
	nc  *nats.Conn
	js  nats.JetStreamContext

	mu     sync.RWMutex
	routes map[string]NatsxRoute         // biz -> route
	subs   map[string]*nats.Subscription // biz -> sub
}

// NewNatsxClient 连接 NATS
func NewNatsxClient(cfg NatsxConfig) (*NatsxClient, error) {
	if len(cfg.Servers) == 0 {
		return nil, errors.New("nats servers missing")
	}
	if cfg.ReconnectWait == 0 {
		cfg.ReconnectWait = 500 * time.Millisecond
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 3 * time.Second
	}
	if cfg.PublishAsyncMax == 0 {
		cfg.PublishAsyncMax = 4096
	}
	opts := []nats.Option{
		nats.Name(cfg.Name),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.ReconnectJitter(100*time.Millisecond, 500*time.Millisecond),
		nats.Timeout(cfg.Timeout),
		nats.UserInfo("nats", "secret"),
	}
	nc, err := nats.Connect(strings.Join(cfg.Servers, ","), opts...)
	if err != nil {
		return nil, err
	}
	return &NatsxClient{
		cfg:    cfg,
		nc:     nc,
		routes: make(map[string]NatsxRoute),
		subs:   make(map[string]*nats.Subscription),
	}, nil
}

// Close 优雅关闭
func (c *NatsxClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for biz, sub := range c.subs {
		_ = sub.Drain()
		delete(c.subs, biz)
	}
	if c.nc != nil {
		return c.nc.Drain()
	}
	return nil
}

// ensureJS 初始化 JetStream 上下文
func (c *NatsxClient) ensureJS() error {
	if c.js != nil {
		return nil
	}
	js, err := c.nc.JetStream(nats.PublishAsyncMaxPending(c.cfg.PublishAsyncMax))
	if err != nil {
		return err
	}
	c.js = js
	return nil
}

// RegisterRoute 注册 Biz 路由
func (c *NatsxClient) RegisterRoute(r NatsxRoute) error {
	if r.Biz == "" || r.Subject == "" {
		return errors.New("invalid route")
	}
	if r.Mode == JetStreamPush || r.Mode == JetStreamPull {
		if err := c.ensureJS(); err != nil {
			return fmt.Errorf("init jetstream: %w", err)
		}
	}
	if r.AckWait == 0 {
		r.AckWait = 30 * time.Second
	}
	if r.MaxAckPending == 0 {
		r.MaxAckPending = 1024
	}
	c.mu.Lock()
	c.routes[r.Biz] = r
	c.mu.Unlock()
	return nil
}

// route 查询已注册路由
func (c *NatsxClient) route(biz string) (NatsxRoute, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.routes[biz]
	return r, ok
}
