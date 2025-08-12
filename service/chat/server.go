package chat

import (
	pb "PProject/gen/message"
	"PProject/service/storage"
	"encoding/base64"
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
	mu       sync.RWMutex
	session  map[string]string // user -> gateway_id
	gateways map[string]*gwStream
}

func NewRouterService() *RouterService {
	return &RouterService{
		session:  make(map[string]string),
		gateways: make(map[string]*gwStream),
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

		// First, write to Redis

		// 1) Write to Redis first
		conv := storage.DMKey(f.GetFrom(), f.GetTo())
		if _, err := storage.AppendStream(conv, map[string]any{
			"from":        f.GetFrom(),
			"to":          f.GetTo(),
			"ts":          f.GetTs(),
			"payload_b64": base64.StdEncoding.EncodeToString(f.GetPayload()),
		}); err != nil {
			log.Printf("redis xadd err: %v", err)
		}

		s.mu.RLock()
		tgtGW := s.session[f.GetTo()]
		s.mu.RUnlock()
		if tgtGW == "" {
			return
		}
		out := &pb.MessageFrame{Type: pb.MessageFrame_DELIVER, From: f.GetFrom(), To: f.GetTo(), Payload: f.GetPayload(), Ts: time.Now().UnixMilli()}
		s.mu.RLock()
		gws := s.gateways[tgtGW]
		println("message :%v", string(out.Payload))
		s.mu.RUnlock()
		if gws == nil {
			return
		}
		_ = gws.stream.Send(out)
	default:
	}
}
