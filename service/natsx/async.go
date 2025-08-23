package natsx

import (
	"context"
	"time"
)

// NatsxAsyncPublisher 简单限速异步发布器
type NatsxAsyncPublisher struct {
	P     *NatsxProducer
	Rate  int           // msgs/sec
	Delay time.Duration // 自定义间隔，优先级高于 Rate
}

func (ap *NatsxAsyncPublisher) Run(ctx context.Context, biz string, payload []byte, hdr map[string]string) error {
	interval := ap.Delay
	if interval == 0 && ap.Rate > 0 {
		interval = time.Second / time.Duration(ap.Rate)
	}
	if interval == 0 {
		interval = time.Millisecond
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			_ = ap.P.Publish(ctx, biz, payload, hdr)
		}
	}
}
