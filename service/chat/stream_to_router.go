package chat

import (
	pb "PProject/gen/message"
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"time"
)

type Server struct {
	gwID       string
	routerAddr string
	reg        *registry
	// below fields are used by ws_server for writing back to clients
	incoming chan *pb.MessageFrame // frames from router destined to local users
}

func NewServer(gwID, routerAddr string) (*Server, error) {
	return &Server{
		gwID: gwID, routerAddr: routerAddr, reg: newRegistry(), incoming: make(chan *pb.MessageFrame, 4096),
	}, nil
}

func (s *Server) RunToRouter() {
	retry := time.Second
	for {
		if err := s.loopRouter(); err != nil {
			log.Printf("router stream closed: %v, retry in %v", err, retry)
			time.Sleep(retry)
			if retry < 5*time.Second {
				retry *= 2
			}
		}
	}
}

func (s *Server) loopRouter() error {
	ctx := context.Background()
	cc, err := grpc.DialContext(ctx, s.routerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer func(cc *grpc.ClientConn) {
		err := cc.Close()
		if err != nil {

		}
	}(cc)
	client := pb.NewRouterClient(cc)
	stream, err := client.Gateway(ctx)
	if err != nil {
		return err
	}

	// announce ourselves (PING)
	_ = stream.Send(&pb.MessageFrame{Type: pb.MessageFrame_PING, GatewayId: s.gwID, Ts: time.Now().UnixMilli()})

	// reader: frames from router -> local incoming
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			f, err := stream.Recv()
			if err != nil {
				return
			}
			if f.GetType() == pb.MessageFrame_DELIVER {
				s.incoming <- f
			}
		}
	}()

	// writer: ws_server will push REGISTER/UNREGISTER/DATA frames via a channel
	for f := range s.outbound() {
		f.GatewayId = s.gwID
		if err := stream.Send(f); err != nil {
			return err
		}
	}
	<-done
	return nil
}

// outbound returns a read-only channel that ws_server pushes into
func (s *Server) outbound() <-chan *pb.MessageFrame { return wsOutbound }

// package-scope channel shared with ws_server.go for simplicity
var wsOutbound = make(chan *pb.MessageFrame, 8192)
