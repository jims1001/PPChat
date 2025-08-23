package main

import (
	pb "PProject/gen/gateway"
	global "PProject/global"
	"PProject/module/user"
	"PProject/service/chat"
	"PProject/service/storage"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"log"
	"net"
	"os"
)

func main() {

	// 配置生成的ids

	global.ConfigIds()

	// 1) Prepare parameters
	gwID := os.Getenv("GATEWAY_ID")
	if gwID == "" {
		gwID = "msg_gw-1"
	}
	routerAddr := os.Getenv("ROUTER_ADDR")
	if routerAddr == "" {
		routerAddr = "127.0.0.1:50051"
	}

	// 2) Init Redis
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
		log.Fatalf("init redis: %v", err)
	}

	conn := chat.NewConnManager()

	// 3) Create gateway instance
	g, err := chat.NewServer(gwID, routerAddr, conn)
	if err != nil {
		log.Fatal(err)
	}

	// 4) Start gRPC service
	go func() {
		lis, err := net.Listen("tcp", ":50052")
		if err != nil {
			log.Fatalf("gRPC listen failed: %v", err)
		}
		gs := grpc.NewServer()

		// Register gateway gRPC service
		pb.RegisterGatewayControlServer(gs, chat.NewMsgGatewayService(g, conn))

		// Register health check service
		healthServer := health.NewServer()
		healthpb.RegisterHealthServer(gs, healthServer)
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
		healthServer.SetServingStatus("gateway.GatewayControl", healthpb.HealthCheckResponse_SERVING)

		log.Println("[gRPC] Listening on :50052")
		if err := gs.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// 5) Start router stream (bi-directional streaming)
	go g.RunToRouter()

	// 6) Start HTTP + WebSocket
	r := gin.New()
	r.GET("/chat", g.HandleWS) // e.g. ws://localhost:8080/chat?user=A&to=B
	r.POST("/login", user.HandlerLogin)
	log.Println("[HTTP] Listening on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
