package main

import (
	pb "PProject/gen/message"
	"PProject/global"
	"PProject/logger"
	"PProject/service/chat"
	"PProject/service/natsx"
	"PProject/service/rpc"
	"PProject/service/storage"
	"net"
	"os"
	"time"

	msgcli "PProject/module/message"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func main() {

	global.ConfigIds()
	global.ConfigMgo()
	global.ConfigRedis()
	global.ConfigKafka(msgcli.HandlerTopicMessage)

	// Initialize the RPC msg handling service
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Errorf(err)
	}

	// Initialize Redis
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:7001"
	}

	password := os.Getenv("REDIS_PASSWORD")
	if password == "" {
		password = "password"
	}

	if err := storage.InitRedis(storage.Config{
		Addr: addr, Password: password, DB: 0,
	}); err != nil {
		logger.Errorf("init redis: %v", err)
	}

	manager := rpc.NewManager(rpc.Config{
		Target:              "localhost:50052",
		DialTimeout:         5 * time.Second,
		HealthCheckInterval: 10 * time.Second,
		PingInterval:        30 * time.Second,
	})

	manager.Start()

	natsx.UseGlobalMiddlewares(natsx.NatsxIdemMiddleware(natsx.NewMemIdem(2*time.Minute), 2*time.Minute))

	// 2) 注册路由（各节点同样的 Durable/Queue）
	_ = natsx.RegisterRoute(natsx.NatsxRoute{
		Biz:           "inbox",
		Subject:       "inbox.user.*",
		Mode:          natsx.JetStreamPush,
		Durable:       "inbox-dur",     // 全节点一致
		Queue:         "inbox-workers", // 全节点一致
		AckWait:       30 * time.Second,
		MaxAckPending: 4096,
	})

	// 3) 注册处理器（可在 StartNats 前/后）
	_ = natsx.RegisterHandler("inbox", func(ctx context.Context, m natsx.NatsxMessage) error {
		// 你的幂等业务逻辑（数据库 UPSERT 等）
		return nil
	})

	// 4) 启动全局 NATS
	natsx.StartNats(natsx.NatsxConfig{
		Servers: []string{"nats://127.0.0.1:4222"},
		Name:    "gw-node-1",
	})

	gs := grpc.NewServer()
	pb.RegisterRouterServer(gs, chat.NewRouterService(manager))
	logger.Infof("router listening :50051")
	if err := gs.Serve(lis); err != nil {
		logger.Errorf(err)
	}

}
