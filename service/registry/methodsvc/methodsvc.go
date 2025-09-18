package methodsvc

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
)

type Opt struct {
	ConsulAddr    string
	Addr          string
	Port          int
	InstanceID    string        // 物理实例ID；方法服务ID = InstanceID#Method
	TTL           time.Duration // TTL 模式（如 10s）。若用 gRPC 健康检查可置 0
	UseGRPCHealth bool          // true 用 GRPC 健康检查（无需 TTL 心跳）
}

type MethodService struct {
	cli        *api.Client
	addr       string
	port       int
	instanceID string
	ttl        time.Duration
	useGRPC    bool

	mu   sync.Mutex
	regs map[string]*api.AgentServiceRegistration // key = methodName
	stop chan struct{}
}

func New(opt Opt) (*MethodService, error) {
	cfg := api.DefaultConfig()
	if opt.ConsulAddr != "" {
		cfg.Address = opt.ConsulAddr
	}
	cli, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	if opt.TTL <= 0 && !opt.UseGRPCHealth {
		opt.TTL = 10 * time.Second
	}
	return &MethodService{
		cli:        cli,
		addr:       opt.Addr,
		port:       opt.Port,
		instanceID: opt.InstanceID,
		ttl:        opt.TTL,
		useGRPC:    opt.UseGRPCHealth,
		regs:       map[string]*api.AgentServiceRegistration{},
		stop:       make(chan struct{}),
	}, nil
}

// Upsert 注册/更新一个“方法服务”（服务名= chat.<Method>），写入完整 Meta（覆盖）
func (m *MethodService) Upsert(methodName string, meta map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if meta == nil {
		meta = map[string]string{}
	}
	meta["method"] = methodName // 方便排查

	svcName := "chat." + methodName // 例：chat.GetChatServiceTopic
	svcID := fmt.Sprintf("%s#%s", m.instanceID, methodName)

	check := &api.AgentServiceCheck{}
	if m.useGRPC {
		check.GRPC = fmt.Sprintf("%s:%d", m.addr, m.port)
		check.Interval = "5s"
		check.Timeout = "2s"
	} else {
		check.TTL = fmt.Sprintf("%ds", int(m.ttl.Seconds()))
		check.DeregisterCriticalServiceAfter = "1m"
	}

	reg := &api.AgentServiceRegistration{
		Name:    svcName,
		ID:      svcID,
		Address: m.addr,
		Port:    m.port,
		Meta:    meta,
		Check:   check,
	}
	if err := m.cli.Agent().ServiceRegister(reg); err != nil {
		return err
	}
	m.regs[methodName] = reg
	return nil
}

// UpdateMetaReplace：用新 meta 覆盖（同ID二次注册，幂等更新）
func (m *MethodService) UpdateMetaReplace(methodName string, meta map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	reg, ok := m.regs[methodName]
	if !ok {
		return fmt.Errorf("method %s not registered", methodName)
	}
	cp := *reg
	if meta == nil {
		meta = map[string]string{}
	}
	meta["method"] = methodName
	cp.Meta = meta
	if err := m.cli.Agent().ServiceRegister(&cp); err != nil {
		return err
	}
	m.regs[methodName] = &cp
	return nil
}

// UpdateMetaPatch：增量更新（仅修改/新增部分键）
func (m *MethodService) UpdateMetaPatch(methodName string, patch map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	reg, ok := m.regs[methodName]
	if !ok {
		return fmt.Errorf("method %s not registered", methodName)
	}
	cp := *reg
	if cp.Meta == nil {
		cp.Meta = map[string]string{}
	}
	for k, v := range patch {
		cp.Meta[k] = v
	}
	if _, has := cp.Meta["method"]; !has {
		cp.Meta["method"] = methodName
	}
	return m.cli.Agent().ServiceRegister(&cp)
}

// 仅在 TTL 健康检查模式下需要
func (m *MethodService) StartTTLHeartbeat() {
	if m.useGRPC {
		return
	}
	interval := m.ttl / 2
	if interval < time.Second {
		interval = time.Second
	}
	go func() {
		tk := time.NewTicker(interval)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				m.mu.Lock()
				for _, reg := range m.regs {
					_ = m.cli.Agent().UpdateTTL("service:"+reg.ID, "pass", api.HealthPassing)
				}
				m.mu.Unlock()
			case <-m.stop:
				return
			}
		}
	}()
}

func (m *MethodService) Stop() {
	close(m.stop)
	// 可选：优雅反注册
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, reg := range m.regs {
		_ = m.cli.Agent().ServiceDeregister(reg.ID)
	}
	m.regs = map[string]*api.AgentServiceRegistration{}
}
