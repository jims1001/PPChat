package main

import (
	chat "PProject/service/chat"
	"PProject/service/storage"
	"github.com/gin-gonic/gin"
	"log"
	"os"
)

func main() {
	// 1) Prepare parameters
	gwID := os.Getenv("GATEWAY_ID") // Current gateway instance ID
	if gwID == "" {
		gwID = "gw-1"
	}
	routerAddr := os.Getenv("ROUTER_ADDR") // Router service address
	if routerAddr == "" {
		routerAddr = "127.0.0.1:50051"
	}

	// Initialize Redis
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	if err := storage.InitRedis(storage.Config{
		Addr: addr, Password: os.Getenv("REDIS_PASS"), DB: 0,
	}); err != nil {
		log.Fatalf("init redis: %v", err)
	}

	// 2) Create a gateway instance (this is your NewServer)
	g, err := chat.NewServer(gwID, routerAddr)
	if err != nil {
		log.Fatal(err)
	}

	// 3) Start the bidirectional stream with Router (internally sends/receives MessageFrame)
	go g.RunToRouter()

	// 4) Start the HTTP/WS service and bind the instance method to the route (must use g.HandleWS)
	r := gin.New()
	r.GET("/chat", g.HandleWS) // ws://localhost:8080/chat?user=A&to=B  (demo usage)
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
