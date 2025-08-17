package main

import (
	pb "PProject/gen/message"
	"PProject/service/chat"
	"PProject/service/rpc"
	"PProject/service/storage"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"time"
)

func main() {
	// Initialize the RPC message handling service
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
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
		log.Fatalf("init redis: %v", err)
	}

	manager := rpc.NewManager(rpc.Config{
		Target:              "localhost:50052",
		DialTimeout:         5 * time.Second,
		HealthCheckInterval: 10 * time.Second,
		PingInterval:        30 * time.Second,
	})

	manager.Start()

	gs := grpc.NewServer()
	pb.RegisterRouterServer(gs, chat.NewRouterService(manager))
	log.Println("router listening :50051")
	if err := gs.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
