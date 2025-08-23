package natsx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// 生成随机 msgID（16字节）
func genMsgID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// PublishOnce：带 Nats-Msg-Id 的发布（JetStream 推荐使用）
// - msgID 为空则自动生成
func (p *NatsxProducer) PublishOnce(ctx context.Context, biz string, data []byte, hdr map[string]string, msgID string) error {
	if hdr == nil {
		hdr = map[string]string{}
	}
	if msgID == "" {
		msgID = genMsgID()
	}
	hdr["Nats-Msg-Id"] = msgID
	return p.Publish(ctx, biz, data, hdr)
}
