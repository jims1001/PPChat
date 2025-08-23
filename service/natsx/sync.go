package natsx

import (
	"context"
	"time"
)

// NatsxSyncPublisher 同步发布器（带重试）
type NatsxSyncPublisher struct {
	P       *NatsxProducer
	Retries int
	Backoff time.Duration
}

func (sp *NatsxSyncPublisher) Publish(ctx context.Context, biz string, payload []byte, hdr map[string]string) error {
	var err error
	for i := 0; i <= sp.Retries; i++ {
		err = sp.P.Publish(ctx, biz, payload, hdr)
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sp.Backoff):
		}
	}
	return err
}
