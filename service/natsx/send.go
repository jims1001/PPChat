package natsx

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
)

func (c *NatsxClient) ToHeader(h map[string]string) nats.Header {
	if len(h) == 0 {
		return nil
	}
	hd := nats.Header{}
	for k, v := range h {
		hd.Add(k, v)
	}
	return hd
}

func (c *NatsxClient) sendCore(subject string, data []byte, hdr map[string]string) error {
	// 用 NewMsg 构造更安全
	msg := nats.NewMsg(subject)
	msg.Data = data

	// 转换 header
	for k, v := range hdr {
		msg.Header.Add(k, v)
	}

	// 直接发送
	if err := c.nc.PublishMsg(msg); err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	return nil
}

func (c *NatsxClient) sendJS(ctx context.Context, subject string, data []byte, hdr map[string]string) error {
	msg := nats.NewMsg(subject)
	msg.Data = data

	// 加 header
	for k, v := range hdr {
		msg.Header.Add(k, v)
	}

	// 带上下文 publish
	ack, err := c.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	fmt.Printf("Published to stream=%s msgflow=%d\n", ack.Stream, ack.Sequence)
	return nil
}
