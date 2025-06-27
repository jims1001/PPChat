package main

import (
	v1 "PProject/gen"
	"context"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
)

type HelloServiceImpl struct {
	v1.UnimplementedHelloServiceServer // 推荐嵌入，防止未来 break
}

func (s *HelloServiceImpl) SayHello(ctx context.Context, req *v1.HelloRequest) (*v1.HelloReply, error) {
	reply := &v1.HelloReply{
		Message: "Hello, " + req.Name,
	}
	return reply, nil
}

func main() {

	go func() {
		lis, err := net.Listen("tcp", ":50051")
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		grpcServer := grpc.NewServer()
		v1.RegisterHelloServiceServer(grpcServer, &HelloServiceImpl{})
		log.Println("gRPC server listening on :50051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	mux := runtime.NewServeMux()
	//opts := []grpc.DialOption{
	//	grpc.WithTransportCredentials(insecure.NewCredentials()),
	//}

	if err := v1.RegisterHelloServiceHandlerServer(context.Background(), mux, &HelloServiceImpl{}); err != nil {
		log.Fatalf("failed to register HTTP handler: %v", err)
	}

	//err := v1.RegisterHelloServiceHandlerFromEndpoint(context.Background(), mux, "localhost:50051", opts)
	//if err != nil {
	//	log.Fatalf("failed to register gateway: %v", err)
	//}

	log.Println("REST gateway listening on :30000")
	if err := http.ListenAndServe(":30000", mux); err != nil {
		log.Fatalf("HTTP server failed to start: %v", err)
	}

}
