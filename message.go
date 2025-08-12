package main

import (
	pb "PProject/gen/message"
	"PProject/service/chat"
	"PProject/service/storage"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
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
		addr = "127.0.0.1:6379"
	}
	if err := storage.InitRedis(storage.Config{
		Addr: addr, Password: os.Getenv("REDIS_PASS"), DB: 0,
	}); err != nil {
		log.Fatalf("init redis: %v", err)
	}

	gs := grpc.NewServer()
	pb.RegisterRouterServer(gs, chat.NewRouterService())
	log.Println("router listening :50051")
	if err := gs.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
