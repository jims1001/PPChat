package main

import (
	pb "PProject/gen/gateway"
	"PProject/global/config"
	"PProject/logger"
	"PProject/module/message/handler"
	"PProject/service/chat"
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

	// 配置为 网关节点
	config.Global.NodeType = config.NodeTypeMsgGateWay
	config.Global = config.MessageGatewayConfig
	config.ConfigRegistry("chat-service", config.Global.NodeId)
	config.ConfigIds()
	config.ConfigRedis()
	config.ConfigMgo()
	config.ConfigMiddleware()
	config.ConfigKafka(msg.HandlerTopicMessage)

	// 延迟获取

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
