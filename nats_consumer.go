package main

import (
	"PProject/service/natsx"
	tools "PProject/tools"
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// 环境变量：
// NATS_SERVERS   (默认 nats://127.0.0.1:4222)
// NATS_NAME      (默认 consumer-1)
// BIZ            (必填)
// SUBJECT        (必填)
// MODE           (core | js_push | js_pull) — 决定订阅方式
// QUEUE          (可选，同组分摊；多节点务必一致)
// DURABLE        (可选，JS durable；多节点务必一致)
// ACK_WAIT_MS    (默认 30000)
// MAX_ACK_PENDING(默认 2048)
// IDEM_TTL_MS    (默认 120000，用于幂等窗口)
// PULL_BATCH     (默认 128，js_pull 用)
// PULL_WAIT_MS   (默认 500，js_pull 用)

func main() {
	servers := strings.Split(tools.GetEnv("NATS_SERVERS", "nats://127.0.0.1:4222"), ",")
	name := tools.GetEnv("NATS_NAME", "consumer-1")
	biz := tools.GetEnv("BIZ", "chat")
	subj := tools.GetEnv("SUBJECT", "send_msg")
	modeStr := tools.GetEnv("MODE", "core")
	queue := tools.GetEnv("QUEUE", "")
	durable := tools.GetEnv("DURABLE", "")
	ackMs := tools.GetEnvInt("ACK_WAIT_MS", 30000)
	maxAck := tools.GetEnvInt("MAX_ACK_PENDING", 2048)
	idemTTL := tools.GetEnvInt("IDEM_TTL_MS", 120000)
	pullBatch := tools.GetEnvInt("PULL_BATCH", 128)
	pullWait := tools.GetEnvInt("PULL_WAIT_MS", 500)

	if biz == "" || subj == "" {
		log.Fatalf("BIZ and SUBJECT are required")
	}

	// 全局中间件：幂等（内存版；生产可替换为 Redis 版）
	natsx.UseGlobalMiddlewares(
		natsx.NatsxIdemMiddleware(natsx.NewMemIdem(time.Duration(idemTTL)*time.Millisecond), time.Duration(idemTTL)*time.Millisecond),
	)

	// 注册路由（可在 StartNats 前调用）
	_ = natsx.RegisterRoute(natsx.NatsxRoute{
		Biz: biz, Subject: subj, Mode: tools.ParseMode(modeStr),
		Queue: queue, Durable: durable,
		AckWait:       time.Duration(ackMs) * time.Millisecond,
		MaxAckPending: maxAck,
	})

	// 启动 NATS
	natsx.StartNats(natsx.NatsxConfig{
		Servers: servers,
		Name:    name,
	})

	// 根据模式选择订阅方式
	switch tools.ParseMode(modeStr) {
	case natsx.Core, natsx.JetStreamPush:
		// Push/回调
		if err := natsx.RegisterHandler(biz, func(ctx context.Context, m natsx.NatsxMessage) error {
			// TODO: 你的业务逻辑；这里演示打印
			log.Printf("[recv] biz=%s subj=%s len=%d hdr=%v", biz, m.Subject, len(m.Data), m.Header)
			return nil
		}); err != nil {
			log.Fatalf("register handler err: %v", err)
		}
	case natsx.JetStreamPull:
		// 拉批处理
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			if err := natsx.PullConsume(ctx, biz, pullBatch, time.Duration(pullWait)*time.Millisecond, func(ctx context.Context, m natsx.NatsxMessage) error {
				log.Printf("[pull] biz=%s subj=%s len=%d hdr=%v", biz, m.Subject, len(m.Data), m.Header)
				return nil
			}); err != nil {
				log.Printf("pull consume err: %v", err)
			}
		}()
	default:
		log.Fatalf("unknown MODE: %s", modeStr)
	}

	log.Printf("[consumer] start servers=%v biz=%s subject=%s mode=%s queue=%s durable=%s",
		servers, biz, subj, modeStr, queue, durable)

	// 优雅退出
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	_ = natsx.StopNats()
	log.Println("consumer exit")
}
