package main

import (
	pb "PProject/gen/gateway"
	"PProject/global"
	"PProject/logger"
	mid "PProject/middleware"
	"PProject/module/message/handler"
	"PProject/module/user"
	"PProject/service/chat"
	"fmt"
	"log"
	"net"
	"os"

	msg "PProject/module/message"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {

	// 配置生成的ids

	// 配置为 网关节点
	global.GlobalConfig.NodeType = global.NodeTypeDataNode
	global.GlobalConfig = global.MessageDataConfig

	global.ConfigIds()
	global.ConfigRedis()
	global.ConfigMgo()
	global.ConfigMiddleware()
	global.ConfigKafka()

	//mid.Manager().Add(midsec.Middleware(midsec.DefaultOptions()))

	// 1) Prepare parameters
	gwID := os.Getenv("GATEWAY_ID")
	if gwID == "" {
		gwID = global.GlobalConfig.GatewayNodeId
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
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", global.GlobalConfig.GrpcPort))
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

		logger.Infof("[gRPC] Listening on :%d", global.GlobalConfig.GrpcPort)
		if err := gs.Serve(lis); err != nil {
			logger.Errorf("gRPC server failed: %v", err)
		}
	}()

	// 5) Start router stream (bi-directional streaming)
	go g.RunToRouter()

	// 6) Start HTTP + WebSocket
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/chat", g.HandleWS) // e.g. ws://localhost:8080/chat?user=A&to=B
	mid.POST(r, "/login", user.HandlerLogin, mid.RouteOpt{IsAuth: false})
	mid.POST(r, "/check", user.HandlerCheck, mid.RouteOpt{IsAuth: true})
	mid.POST(r, "/user", user.HandleUserInfo, mid.RouteOpt{IsAuth: true})
	//r.POST("/check", user.HandlerCheck)
	//r.POST("/user")

	logger.Infof("[HTTP] Listening on :%d", global.GlobalConfig.Port)
	if err := r.Run(fmt.Sprintf(":%d", global.GlobalConfig.Port)); err != nil {
		logger.Errorf("HTTP server failed: %v", err)
	}
}
