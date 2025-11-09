package main

import (
	pb "PProject/gen/gateway"
	"PProject/global/config"
	"PProject/logger"
	mid "PProject/middleware"
	"PProject/middleware/security"
	msg "PProject/module/message"

	chatService "PProject/module/chat/service"
	"PProject/module/user"
	"PProject/service/chat"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {

	// 配置生成的ids

	// 配置为 网关节点
	config.Global.NodeType = config.NodeTypeApiNode
	log.Println("config.Global.NodeType ")
	config.Global = config.MessageApiNodeConfig
	config.ConfigIds()
	config.ConfigRedis()
	config.ConfigMgo()
	config.ConfigMiddleware()

	//TODO 先注释掉
	//config.ConfigKafka(msg.HandlerTopicMessage)

	// 1) Prepare parameters
	gwID := os.Getenv("GATEWAY_ID")
	if gwID == "" {
		gwID = config.Global.NodeId
	}
	routerAddr := os.Getenv("ROUTER_ADDR")
	if routerAddr == "" {
		routerAddr = fmt.Sprintf("127.0.0.1:%d", config.Global.GrpcPort)
	}

	conn := chat.NewConnManager(gwID)

	// 3) Create gateway instance
	g, err := chat.NewServer(gwID, routerAddr, conn, msg.MessageProducerHandler)
	if err != nil {
		log.Fatal(err)
	}

	chatCtx := &chat.ChatContext{S: g}

	err = g.Disp().Run(chatCtx)
	if err != nil {
		logger.Errorf("error is %v", err)
		return
	}

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
	r.Use(security.CORSMiddleware(&security.CORSOptions{
		AllowOrigins: []string{
			"http://localhost:5173", // 你的前端
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "AuthorizationHash"},
		AllowCredentials: true,
	}))
	r.Use(gin.Recovery())
	mid.POST(r, "/login", user.HandlerLogin, mid.RouteOpt{IsAuth: false})
	mid.POST(r, "/check", user.HandlerCheck, mid.RouteOpt{IsAuth: true})
	mid.GET(r, "/user", user.HandleUserInfo, mid.RouteOpt{IsAuth: true})
	mid.POST(r, "/chat/history", chatService.HandlerListMessages, mid.RouteOpt{IsAuth: true})
	mid.POST(r, "/chat/conversation", chatService.HandlerListConversations, mid.RouteOpt{IsAuth: true})

	logger.Infof("[HTTP] Listening on :%d", config.Global.Port)
	if err := r.Run(fmt.Sprintf(":%d", config.Global.Port)); err != nil {
		logger.Errorf("HTTP server failed: %v", err)
	}
}
