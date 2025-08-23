package tools

import (
	"PProject/service/natsx"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
)

// 环境变量：
// NATS_SERVERS   (默认 nats://127.0.0.1:4222)
// NATS_NAME      (默认 producer-1)
// BIZ            (必填，如 inbox / chat / jobs)
// SUBJECT        (必填，对应该 Biz 的 subject)
// MODE           (core | js_push | js_pull) 生产端对 js_pull 与 js_push 调用一致
// QUEUE          (可选，同组分摊用；生产端仅注册使用)
// DURABLE        (可选，JS durable；在 js_push/js_pull 场景建议配置)
// ACK_WAIT_MS    (默认 30000)
// MAX_ACK_PENDING(默认 1024)
// PUB_RATE       (msgs/sec，默认 10；<=0 则尽快发)
// MSG            (默认 "hello")
// USE_ONCE       (true 则使用 PublishOnce 并自动追加 Nats-Msg-Id)
// HDR            (可选，形如 k1=v1,k2=v2)

func GetEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
func GetEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}
func GetEnvBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return def
	}
	return v == "true" || v == "1" || v == "yes"
}

func ParseMode(s string) natsx.NatsxMode {
	switch strings.ToLower(s) {
	case "core":
		return natsx.Core
	case "js_push":
		return natsx.JetStreamPush
	case "js_pull":
		return natsx.JetStreamPull
	default:
		return natsx.Core
	}
}

func ParseHdr(s string) map[string]string {
	if s == "" {
		return nil
	}
	out := map[string]string{}
	parts := strings.Split(s, ",")
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) == 2 && kv[0] != "" {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

func RandMsgID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
