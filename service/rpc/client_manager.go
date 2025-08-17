package rpc

import (
	pb "PProject/gen/gateway"
	"context"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Config struct {
	Target              string        // gRPC service address
	DialTimeout         time.Duration // connection timeout
	HealthCheckInterval time.Duration // health check interval
	PingInterval        time.Duration // PING message interval
}

type Manager struct {
	cfg       Config
	mu        sync.RWMutex
	conn      *grpc.ClientConn
	client    pb.GatewayControlClient
	healthy   bool
	stopCh    chan struct{}
	startOnce sync.Once
}

func NewManager(cfg Config) *Manager {
	return &Manager{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

func (m *Manager) Start() {
	m.startOnce.Do(func() {
		go m.run()
	})
}

func (m *Manager) run() {
	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		if err := m.connect(); err != nil {
			log.Printf("[gwctrl] connect failed: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// health check loop
		go m.healthLoop()
		// periodic ping loop
		go m.pingLoop()

		return
	}
}

func (m *Manager) connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.cfg.DialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, m.cfg.Target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return err
	}

	client := pb.NewGatewayControlClient(conn)

	m.mu.Lock()
	m.conn = conn
	m.client = client
	m.healthy = true
	m.mu.Unlock()

	log.Printf("[gwctrl] connected to %s", m.cfg.Target)
	return nil
}

func (m *Manager) healthLoop() {
	ticker := time.NewTicker(m.cfg.HealthCheckInterval)
	defer ticker.Stop()

	health := grpc_health_v1.NewHealthClient(m.conn)

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			resp, err := health.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: "gateway.GatewayControl"})
			if err != nil || resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
				log.Printf("[gwctrl] health check failed: %v", err)
				m.reconnect()
				return
			}
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) pingLoop() {
	ticker := time.NewTicker(m.cfg.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.mu.RLock()
			client := m.client
			m.mu.RUnlock()

			if client == nil {
				continue
			}

			_, err := client.Send(context.Background(), &pb.SendMessageFrame{
				Type: pb.SendMessageFrame_PING,
				Ts:   time.Now().UnixMilli(),
			})
			if err != nil {
				log.Printf("[gwctrl] ping failed: %v", err)
				m.reconnect()
				return
			}
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) reconnect() {
	m.mu.Lock()
	if m.conn != nil {
		_ = m.conn.Close()
	}
	m.conn = nil
	m.client = nil
	m.healthy = false
	m.mu.Unlock()

	go m.run()
}

func (m *Manager) Stop() {
	close(m.stopCh)
	m.mu.Lock()
	if m.conn != nil {
		_ = m.conn.Close()
	}
	m.conn = nil
	m.client = nil
	m.mu.Unlock()
}

func (m *Manager) Send(ctx context.Context, frame *pb.SendMessageFrame) error {
	m.mu.RLock()
	client := m.client
	m.mu.RUnlock()

	if client == nil {
		return grpc.ErrClientConnClosing
	}

	_, err := client.Send(ctx, frame)
	return err
}
