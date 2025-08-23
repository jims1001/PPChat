package natsx

import (
	"context"
	"fmt"
	"time"
)

// NatsManager 统一门面：对外只暴露这一个对象来用
type NatsManager struct {
	client   *NatsxClient
	producer *NatsxProducer
	consumer *NatsxConsumer
}

// NewNatsManager 初始化
func NewNatsManager(cfg NatsxConfig, middlewares ...NatsxMiddleware) (*NatsManager, error) {
	c, err := NewNatsxClient(cfg)
	if err != nil {
		return nil, err
	}
	m := &NatsManager{
		client:   c,
		producer: NewNatsxProducer(c),
		consumer: NewNatsxConsumer(c, middlewares...),
	}
	return m, nil
}

// Close 释放资源（优雅关闭订阅与连接）
func (m *NatsManager) Close() error {
	if m == nil || m.client == nil {
		return nil
	}
	return m.client.Close()
}

// RegisterRoute 注册业务路由（biz -> subject / mode / queue / durable ...）
func (m *NatsManager) RegisterRoute(r NatsxRoute) error {
	if m == nil || m.client == nil {
		return fmt.Errorf("manager not initialized")
	}
	return m.client.RegisterRoute(r)
}

// Publish 生产消息（按 biz 路由）
func (m *NatsManager) Publish(ctx context.Context, biz string, data []byte, hdr map[string]string) error {
	if m == nil || m.producer == nil {
		return fmt.Errorf("manager not initialized")
	}
	return m.producer.Publish(ctx, biz, data, hdr)
}

// PublishOnce 生产消息（带 Nats-Msg-Id 去重）
func (m *NatsManager) PublishOnce(ctx context.Context, biz string, data []byte, hdr map[string]string, msgID string) error {
	if m == nil || m.producer == nil {
		return fmt.Errorf("manager not initialized")
	}
	return m.producer.PublishOnce(ctx, biz, data, hdr, msgID)
}

// Subscribe 订阅（Core/JetStream Push），同组内用 Queue 分摊；广播则 Queue 置空
func (m *NatsManager) Subscribe(biz string, h NatsxHandler) error {
	if m == nil || m.consumer == nil {
		return fmt.Errorf("manager not initialized")
	}
	return m.consumer.Subscribe(biz, h)
}

// PullConsume JetStream Pull 拉批消费（适合后端 worker 池）
func (m *NatsManager) PullConsume(
	ctx context.Context,
	biz string,
	batch int,
	wait time.Duration,
	h NatsxHandler,
) error {
	if m == nil || m.consumer == nil {
		return fmt.Errorf("manager not initialized")
	}
	return m.consumer.PullConsume(ctx, biz, batch, wait, h)
}
