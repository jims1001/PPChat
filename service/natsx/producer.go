package natsx

import (
	"context"
	"fmt"
)

// NatsxProducer 生产端
type NatsxProducer struct{ c *NatsxClient }

func NewNatsxProducer(c *NatsxClient) *NatsxProducer { return &NatsxProducer{c: c} }

// Publish 按 Biz 路由发送
func (p *NatsxProducer) Publish(ctx context.Context, biz string, data []byte, hdr map[string]string) error {
	r, ok := p.c.route(biz)
	if !ok {
		return fmt.Errorf("route not found: %s", biz)
	}
	switch r.Mode {
	case Core:
		return p.c.sendCore(r.Subject, data, hdr)
	case JetStreamPush, JetStreamPull:
		return p.c.sendJS(ctx, r.Subject, data, hdr)
	default:
		return fmt.Errorf("unsupported mode")
	}
}
