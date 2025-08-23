package main

import (
	"PProject/service/natsx"
	"PProject/tools"
	"context"
	"fmt"
	"log"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	servers := strings.Split(tools.GetEnv("NATS_SERVERS", "nats://127.0.0.1:4222"), ",")
	name := tools.GetEnv("NATS_NAME", "producer-1")
	biz := tools.GetEnv("BIZ", "chat")
	subj := tools.GetEnv("SUBJECT", "send_msg")
	modeStr := tools.GetEnv("MODE", "core")
	queue := tools.GetEnv("QUEUE", "")
	durable := tools.GetEnv("DURABLE", "")
	ackMs := tools.GetEnvInt("ACK_WAIT_MS", 30000)
	maxAck := tools.GetEnvInt("MAX_ACK_PENDING", 1024)
	rate := tools.GetEnvInt("PUB_RATE", 1)
	msgBody := tools.GetEnv("MSG", "hello")
	useOnce := tools.GetEnvBool("USE_ONCE", true)
	hdr := tools.ParseHdr(tools.GetEnv("HDR", ""))

	if biz == "" || subj == "" {
		log.Fatalf("BIZ and SUBJECT are required")
	}

	cfg := natsx.NatsxConfig{
		Servers: servers,
		Name:    name,
	}
	nm, err := natsx.NewNatsManager(cfg /* 可挂中间件 */)
	if err != nil {
		log.Fatal(err)
	}
	defer nm.Close()

	// 注册路由（产生端也注册，确保存在）
	err = nm.RegisterRoute(natsx.NatsxRoute{
		Biz: biz, Subject: subj, Mode: tools.ParseMode(modeStr),
		Queue: queue, Durable: durable,
		AckWait:       time.Duration(ackMs) * time.Millisecond,
		MaxAckPending: maxAck,
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	interval := time.Millisecond
	if rate > 0 {
		interval = time.Second / time.Duration(rate)
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	i := 0
	log.Printf("[producer] start servers=%v biz=%s subject=%s rate=%d/s once=%v", servers, biz, subj, rate, useOnce)
	for {
		select {
		case <-ctx.Done():
			log.Println("producer exit")
			return
		case <-t.C:
			i++
			body := fmt.Sprintf("%s #%d @%s", msgBody, i, time.Now().Format(time.RFC3339Nano))
			if useOnce {
				id := tools.RandMsgID() // 真实生产建议：用业务ID，如订单号/消息UUID
				if err := nm.PublishOnce(ctx, biz, []byte(body), hdr, id); err != nil {
					log.Printf("publish once err: %v", err)
				}
			} else {
				if err := nm.Publish(ctx, biz, []byte(body), hdr); err != nil {
					log.Printf("publish err: %v", err)
				}
			}
		}
	}
}
