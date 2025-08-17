package chat

import (
	gateway "PProject/gen/gateway"
	pb "PProject/gen/message"
	rpc "PProject/service/rpc"
	"PProject/service/storage"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

type gwStream struct {
	id     string
	stream pb.Router_GatewayServer
}

type RouterService struct {
	pb.UnimplementedRouterServer
	mu            sync.RWMutex
	session       map[string]string // user -> gateway_id
	gateways      map[string]*gwStream
	clientManager *rpc.Manager
}

func NewRouterService(clientManager *rpc.Manager) *RouterService {
	return &RouterService{
		session:       make(map[string]string),
		gateways:      make(map[string]*gwStream),
		clientManager: clientManager,
	}
}

func (s *RouterService) Gateway(stream pb.Router_GatewayServer) error {
	var gatewayID string
	for {
		in, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if gatewayID == "" {
			gatewayID = in.GetGatewayId()
			if gatewayID == "" {
				gatewayID = "unknown-" + time.Now().Format("150405.000")
			}
			s.registerGateway(gatewayID, stream)
			log.Printf("gateway connected: %s", gatewayID)
		}
		s.handleFrame(gatewayID, in)
	}
}

func (s *RouterService) registerGateway(id string, stream pb.Router_GatewayServer) {
	s.mu.Lock()
	s.gateways[id] = &gwStream{id: id, stream: stream}
	s.mu.Unlock()
}

func (s *RouterService) handleFrame(gw string, f *pb.MessageFrame) {
	switch f.GetType() {
	case pb.MessageFrame_REGISTER:
		s.mu.Lock()
		s.session[f.GetFrom()] = gw
		s.mu.Unlock()
	case pb.MessageFrame_UNREGISTER:
		s.mu.Lock()
		delete(s.session, f.GetFrom())
		s.mu.Unlock()
	case pb.MessageFrame_DATA:
		s.handleDataFrame(f)
	default:
		log.Printf("unsupported frame type: %v", f.GetType())
	}
}

func (s *RouterService) handleDataFrame(f *pb.MessageFrame) {
	// Redis 持久化
	conv := storage.DMKey(f.GetFrom(), f.GetTo())
	_, _ = storage.AppendStream(conv, map[string]any{
		"from":        f.GetFrom(),
		"to":          f.GetTo(),
		"ts":          f.GetTs(),
		"payload_b64": base64.StdEncoding.EncodeToString(f.GetPayload()),
	})

	fmt.Printf("get payload: %s\n", string(f.GetPayload()))

	s.mu.RLock()
	tgtGW := s.session[f.GetTo()]
	gws := s.gateways[tgtGW]
	s.mu.RUnlock()

	if tgtGW == "" || gws == nil {
		log.Printf("target gateway or stream missing for user %s", f.GetTo())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sendFrame := &gateway.SendMessageFrame{
		Type:    gateway.SendMessageFrame_DELIVER,
		From:    f.GetFrom(),
		To:      f.GetTo(),
		Payload: f.GetPayload(),
		Ts:      time.Now().UnixMilli(),
	}
	if err := s.clientManager.Send(ctx, sendFrame); err != nil {
		log.Printf("clientManager send error: %v", err)
	}
}
