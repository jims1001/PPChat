package natsx

import "golang.org/x/net/context"

// NatsxMessage 统一消息对象
type NatsxMessage struct {
	Subject string
	Data    []byte
	Header  map[string]string
}

// NatsxHandler 业务处理函数
type NatsxHandler func(ctx context.Context, msg NatsxMessage) error

// NatsxMiddleware 中间件（日志、指标、重试等）
type NatsxMiddleware func(NatsxHandler) NatsxHandler

// NatsxChain 组合中间件
func NatsxChain(h NatsxHandler, mws ...NatsxMiddleware) NatsxHandler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
