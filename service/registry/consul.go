package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/consul/api"
)

type ConsulRegistry struct {
	cli *api.Client
}

func NewConsul(addr string) (*ConsulRegistry, error) {
	cfg := api.DefaultConfig()
	if addr != "" {
		cfg.Address = addr
	}
	cli, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &ConsulRegistry{cli: cli}, nil
}

// 本示例用 TTL 主动上报：不需要 HTTP 健康检查
func (r *ConsulRegistry) Register(ctx context.Context, inst Instance, _ RegisterOptions) error {
	check := &api.AgentServiceCheck{
		TTL:                            "10s", // TTL 窗口（上报要小于这个间隔）
		DeregisterCriticalServiceAfter: "1m",  // 连续失败 1 分钟后摘除
	}

	reg := &api.AgentServiceRegistration{
		Name:    inst.Service,
		ID:      inst.ID,
		Address: inst.Address,
		Port:    inst.Port,
		Meta:    inst.Metadata,
		Check:   check,
	}
	return r.cli.Agent().ServiceRegister(reg)
}

func (r *ConsulRegistry) Deregister(ctx context.Context, _ string, id string) error {
	return r.cli.Agent().ServiceDeregister(id)
}

func (r *ConsulRegistry) List(ctx context.Context, service string) ([]Instance, error) {
	entries, _, err := r.cli.Health().Service(service, "", true, &api.QueryOptions{RequireConsistent: true})
	if err != nil {
		return nil, err
	}
	out := make([]Instance, 0, len(entries))
	for _, e := range entries {
		out = append(out, Instance{
			Service:  service,
			ID:       e.Service.ID,
			Address:  e.Service.Address,
			Port:     e.Service.Port,
			Metadata: e.Service.Meta,
		})
	}
	return out, nil
}

type consulWatcher struct {
	r       *ConsulRegistry
	service string
	lastIdx uint64
}

func (w *consulWatcher) Next() ([]Instance, error) {
	q := &api.QueryOptions{WaitTime: 10 * time.Minute}
	if w.lastIdx != 0 {
		q.WaitIndex = w.lastIdx
	}
	entries, meta, err := w.r.cli.Health().Service(w.service, "", true, q)
	if err != nil {
		return nil, err
	}
	if meta != nil {
		w.lastIdx = meta.LastIndex
	}
	out := make([]Instance, 0, len(entries))
	for _, e := range entries {
		out = append(out, Instance{
			Service:  w.service,
			ID:       e.Service.ID,
			Address:  e.Service.Address,
			Port:     e.Service.Port,
			Metadata: e.Service.Meta,
		})
	}
	return out, nil
}

func (w *consulWatcher) Stop() error { return nil }

func (r *ConsulRegistry) Watch(ctx context.Context, service string) (Watcher, error) {
	return &consulWatcher{r: r, service: service}, nil
}

func (r *ConsulRegistry) Close() error { return nil }

// Active TTL reporting
// checkID: "service:<service-id>" 或你自定义的独立检查 ID
// status: "pass" | "warn" | "fail"
func (r *ConsulRegistry) AgentUpdateTTL(checkID, output, status string) error {
	return r.cli.Agent().UpdateTTL(checkID, output, status)
}

// Helper（仅当你要改回 HTTP 健康时有用）
func HealthURL(host string, port int) string {
	return fmt.Sprintf("http://%s:%d/health", host, port)
}
