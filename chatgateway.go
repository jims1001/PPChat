package main

import (
	pb "PProject/gen/gateway"
	"PProject/global/config"
	"PProject/logger"
	"PProject/module/message/handler"
	"PProject/service/chat"
	"PProject/service/registry"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	msg "PProject/module/message"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	// 配置生成的ids

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 配置为 网关节点
	config.Global.NodeType = config.NodeTypeMsgGateWay
	config.Global = config.MessageGatewayConfig

	config.ConfigIds()
	config.ConfigRedis()
	config.ConfigMgo()
	config.ConfigMiddleware()
	config.ConfigKafka(msg.HandlerTopicMessage)

	consulAddr := os.Getenv("CONSUL_ADDR") // 例如：127.0.0.1:8500
	if consulAddr == "" {
		consulAddr = "127.0.0.1:8500"
	}

	// 1) 初始化 Registry + Manager
	reg, err := registry.NewConsul(consulAddr)
	must(err)
	sman := registry.New(reg, 20*time.Second, log.Default())

	// 2) 注册本服务（只需可路由的地址/端口；TTL 主动上报，不走 HTTP 健康）
	inst := registry.Instance{
		Service:  "gateway",
		ID:       "gateway-01",
		Address:  "127.0.0.1", // TTL 模式下不影响 Consul 健康探测
		Port:     8080,
		Metadata: map[string]string{"zone": "az1", "weight": "2", "nodeId": "gateway_01"},
	}

	must(sman.RegisterSelf(ctx, inst, registry.RegisterOptions{}))
	defer sman.DeregisterSelf(ctx, inst.Service, inst.ID)

	// 3) 启动 TTL 主动上报（每 5s 上报一次 PASS）
	// 你也可以传 statusFn 动态汇总 Redis/Kafka 状态 → "pass/warn/fail"
	sman.StartTTLReport(ctx, inst.ID, 5*time.Second, nil)

	// 4) 订阅本服务的实例变更（演示）
	must(sman.StartWatch(ctx, "gateway"))

	// 5) 模拟挑选实例（平滑加权轮询）
	for i := 1; i <= 8; i++ {
		time.Sleep(1500 * time.Millisecond)
		if pick, ok := sman.Pick("gateway", nil); ok {
			fmt.Printf("[pick %02d] -> %s:%d (id=%s, zone=%s)\n",
				i, pick.Address, pick.Port, pick.ID, pick.Metadata["zone"])
		} else {
			fmt.Println("[pick] no instance")
		}
	}

	_ = sman.Close()

	// 健康探针（Consul 会调它）

	// 1) Prepare parameters
	gwID := os.Getenv("GATEWAY_ID")
	if gwID == "" {
		gwID = config.Global.NodeId
	}
	routerAddr := os.Getenv("ROUTER_ADDR")
	if routerAddr == "" {
		routerAddr = "127.0.0.1:50051"
	}

	conn := chat.NewConnManager(gwID)

	// 3) Create gateway instance
	g, err := chat.NewServer(gwID, routerAddr, conn, msg.MessageProducerHandler)
	if err != nil {
		log.Fatal(err)
	}

	chatCtx := &chat.ChatContext{S: g}
	g.Disp().Register(handler.NewConnectHandler(chatCtx))
	g.Disp().Register(handler.NewPingHandler(chatCtx))
	g.Disp().Register(handler.NewAuthHandler(chatCtx))
	g.Disp().Register(handler.NewAckHandler(chatCtx))
	g.Disp().Register(handler.NewDataHandler(chatCtx))
	g.Disp().Register(handler.NewCAckHandler(chatCtx))
	g.Disp().Register(handler.NewRelayHandler(chatCtx))

	err = g.Disp().Run(chatCtx)
	if err != nil {
		logger.Errorf("error is %v", err)
		return
	}

	// 4) Start gRPC service
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Global.GrpcPort))
		if err != nil {
			logger.Errorf("gRPC listen failed: %v", err)
		}
		gs := grpc.NewServer()

		// Register gateway gRPC service
		pb.RegisterGatewayControlServer(gs, chat.NewMsgGatewayService(g, conn))

		// Register health check service
		healthServer := health.NewServer()
		healthpb.RegisterHealthServer(gs, healthServer)
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
		healthServer.SetServingStatus("gateway.GatewayControl", healthpb.HealthCheckResponse_SERVING)

		logger.Infof("[gRPC] Listening on :%d", config.Global.GrpcPort)
		if err := gs.Serve(lis); err != nil {
			logger.Errorf("gRPC server failed: %v", err)
		}
	}()

	// 5) Start router stream (bi-directional streaming)
	go g.RunToRouter()

	// 6) Start HTTP + WebSocket
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/chat", g.HandleWS)

	logger.Infof("[HTTP] Listening on :%d", config.Global.Port)
	if err := r.Run(fmt.Sprintf(":%d", config.Global.Port)); err != nil {
		logger.Errorf("HTTP server failed: %v", err)
	}

	for {
		time.Sleep(time.Second)
	}

}
