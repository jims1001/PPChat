package natsx

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// NatsxConsumer 消费端
type NatsxConsumer struct {
	c   *NatsxClient
	mws []NatsxMiddleware
}

func NewNatsxConsumer(c *NatsxClient, mws ...NatsxMiddleware) *NatsxConsumer {
	return &NatsxConsumer{c: c, mws: mws}
}

// Subscribe Core / JetStream Push 订阅（JS 会自动 ACK/NACK）
func (cs *NatsxConsumer) Subscribe(biz string, h NatsxHandler) error {
	r, ok := cs.c.route(biz)
	if !ok {
		return fmt.Errorf("route not found: %s", biz)
	}
	h = NatsxChain(h, cs.mws...)

	switch r.Mode {
	case Core:
		var (
			sub *nats.Subscription
			err error
		)
		cb := func(m *nats.Msg) {
			_ = h(context.Background(), NatsxMessage{
				Subject: m.Subject,
				Data:    append([]byte(nil), m.Data...),
				Header:  headerToMap(m.Header),
			})
		}
		if r.Queue == "" {
			sub, err = cs.c.nc.Subscribe(r.Subject, cb)
		} else {
			sub, err = cs.c.nc.QueueSubscribe(r.Subject, r.Queue, cb)
		}
		if err != nil {
			return err
		}
		_ = sub.SetPendingLimits(1_000_000, 64*1024*1024)
		cs.c.mu.Lock()
		cs.c.subs[biz] = sub
		cs.c.mu.Unlock()
		return nil

	case JetStreamPush:
		if cs.c.js == nil {
			return errors.New("jetstream not initialized")
		}
		opts := []nats.SubOpt{
			nats.ManualAck(),
			nats.AckWait(r.AckWait),
			nats.MaxAckPending(r.MaxAckPending),
		}
		if r.Durable != "" {
			opts = append(opts, nats.Durable(r.Durable))
		}

		cb := func(m *nats.Msg) {
			msg := NatsxMessage{
				Subject: m.Subject,
				Data:    append([]byte(nil), m.Data...),
				Header:  headerToMap(m.Header),
			}
			if err := h(context.Background(), msg); err == nil {
				_ = m.Ack()
			} else {
				_ = m.Nak()
			}
		}

		var (
			sub *nats.Subscription
			err error
		)
		if r.Queue == "" {
			sub, err = cs.c.js.Subscribe(r.Subject, cb, opts...)
		} else {
			sub, err = cs.c.js.QueueSubscribe(r.Subject, r.Queue, cb, opts...)
		}
		if err != nil {
			return err
		}
		cs.c.mu.Lock()
		cs.c.subs[biz] = sub
		cs.c.mu.Unlock()
		return nil

	default:
		return fmt.Errorf("mode not supported in Subscribe: %v", r.Mode)
	}
}

// PullConsume JetStream Pull 拉取消费（批量）
func (cs *NatsxConsumer) PullConsume(ctx context.Context, biz string, batch int, wait time.Duration, h NatsxHandler) error {
	r, ok := cs.c.route(biz)
	if !ok {
		return fmt.Errorf("route not found: %s", biz)
	}
	if r.Mode != JetStreamPull {
		return fmt.Errorf("biz=%s not JetStreamPull", biz)
	}
	if cs.c.js == nil {
		return errors.New("jetstream not initialized")
	}
	if r.Durable == "" {
		return errors.New("JetStreamPull requires Durable consumer name")
	}

	sub, err := cs.c.js.PullSubscribe(r.Subject, r.Durable, nats.PullMaxWaiting(8))
	if err != nil {
		return err
	}
	h = NatsxChain(h, cs.mws...)
	if batch <= 0 {
		batch = 64
	}
	if wait <= 0 {
		wait = 500 * time.Millisecond
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			msgs, err := sub.Fetch(batch, nats.MaxWait(wait))
			if err == nats.ErrTimeout {
				continue
			}
			if err != nil {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			for _, m := range msgs {
				msg := NatsxMessage{
					Subject: m.Subject,
					Data:    append([]byte(nil), m.Data...),
					Header:  headerToMap(m.Header),
				}
				if err := h(context.Background(), msg); err == nil {
					_ = m.Ack()
				} else {
					_ = m.Nak()
				}
			}
		}
	}
}

func headerToMap(h nats.Header) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}
