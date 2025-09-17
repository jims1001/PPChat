package registry

import (
	"context"
	"log"
	"sync"
	"time"
)

type Filter struct {
	Zone    string            // 例: "az1"
	Require map[string]string // 必须包含的元数据键值
}

type serviceState struct {
	mu       sync.RWMutex
	all      []Instance
	lb       *SWRR
	watching bool
	cancel   context.CancelFunc
}

type ServiceManager struct {
	reg        Registry
	statesMu   sync.Mutex
	states     map[string]*serviceState
	refreshTTL time.Duration
	logger     *log.Logger
}

func New(reg Registry, refreshTTL time.Duration, logger *log.Logger) *ServiceManager {
	if refreshTTL <= 0 {
		refreshTTL = 30 * time.Second
	}
	return &ServiceManager{
		reg:        reg,
		states:     make(map[string]*serviceState),
		refreshTTL: refreshTTL,
		logger:     logger,
	}
}

func (m *ServiceManager) ensure(service string) *serviceState {
	m.statesMu.Lock()
	defer m.statesMu.Unlock()
	s, ok := m.states[service]
	if !ok {
		s = &serviceState{lb: NewSWRR()}
		m.states[service] = s
	}
	return s
}

func (m *ServiceManager) StartWatch(ctx context.Context, service string) error {
	st := m.ensure(service)

	st.mu.Lock()
	if st.watching {
		st.mu.Unlock()
		return nil
	}
	wctx, cancel := context.WithCancel(ctx)
	st.cancel = cancel
	st.watching = true
	st.mu.Unlock()

	// 初次列表
	if list, err := m.reg.List(ctx, service); err == nil {
		m.apply(service, list)
	}

	// Watch 线程
	go func() {
		w, err := m.reg.Watch(wctx, service)
		if err != nil {
			m.log("watch start error: %v", err)
			return
		}
		defer w.Stop()

		tk := time.NewTicker(m.refreshTTL)
		defer tk.Stop()

		for {
			select {
			case <-wctx.Done():
				return
			default:
			}

			upd, err := w.Next()
			if err != nil {
				m.log("watch next error: %v", err)
				// 兜底：周期性全量刷新
				select {
				case <-tk.C:
					if lst, e := m.reg.List(context.Background(), service); e == nil {
						m.apply(service, lst)
					}
				case <-wctx.Done():
					return
				}
				continue
			}
			m.apply(service, upd)
		}
	}()
	return nil
}

func (m *ServiceManager) StopWatch(service string) {
	st := m.ensure(service)
	st.mu.Lock()
	if st.cancel != nil {
		st.cancel()
	}
	st.watching = false
	st.mu.Unlock()
}

func (m *ServiceManager) apply(service string, list []Instance) {
	st := m.ensure(service)
	st.mu.Lock()
	defer st.mu.Unlock()
	st.all = list
	st.lb.Update(list)
	m.log("service=%s updated: %d instances", service, len(list))
}

func (m *ServiceManager) All(service string) []Instance {
	st := m.ensure(service)
	st.mu.RLock()
	defer st.mu.RUnlock()
	out := make([]Instance, len(st.all))
	copy(out, st.all)
	return out
}

func (m *ServiceManager) Pick(service string, f *Filter) (Instance, bool) {
	// 无过滤条件：直接用全局 SWRR
	if f == nil || (f.Zone == "" && len(f.Require) == 0) {
		st := m.ensure(service)
		return st.lb.Next()
	}
	// 有过滤：先筛选再临时 SWRR
	all := m.All(service)
	pool := make([]Instance, 0, len(all))
	for _, it := range all {
		if f.Zone != "" && it.Metadata["zone"] != f.Zone && it.Metadata["az"] != f.Zone {
			continue
		}
		ok := true
		for k, v := range f.Require {
			if it.Metadata[k] != v {
				ok = false
				break
			}
		}
		if ok {
			pool = append(pool, it)
		}
	}
	if len(pool) == 0 {
		return Instance{}, false
	}
	tmp := NewSWRR()
	tmp.Update(pool)
	return tmp.Next()
}

func (m *ServiceManager) RegisterSelf(ctx context.Context, inst Instance, opt RegisterOptions) error {
	return m.reg.Register(ctx, inst, opt)
}
func (m *ServiceManager) DeregisterSelf(ctx context.Context, service, id string) error {
	return m.reg.Deregister(ctx, service, id)
}

func (m *ServiceManager) Close() error { return m.reg.Close() }

func (m *ServiceManager) log(format string, args ...any) {
	if m.logger != nil {
		m.logger.Printf(format, args...)
	}
}

// ---------------- TTL 主动上报 ----------------

// statusFn: 返回 (message, state)，state 取 "pass"/"warn"/"fail"
// 若 statusFn == nil，则固定上报 "ok","pass"
func (m *ServiceManager) StartTTLReport(ctx context.Context, serviceID string, interval time.Duration, statusFn func() (string, string)) {
	if interval <= 0 {
		interval = 5 * time.Second // 小于注册的 10s TTL
	}
	// 仅对 Consul 实现有效：拿到 *ConsulRegistry
	cr, ok := m.reg.(*ConsulRegistry)
	if !ok {
		m.log("StartTTLReport: registry is not Consul; skip TTL reporting")
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// 先报一次 PASS，避免 warning
		if statusFn == nil {
			_ = cr.AgentUpdateTTL("service:"+serviceID, "ok", "pass")
		} else if msg, state := statusFn(); state != "" {
			_ = cr.AgentUpdateTTL("service:"+serviceID, msg, state)
		}

		for {
			select {
			case <-ctx.Done():
				// 可选：退出前打一条 warn
				_ = cr.AgentUpdateTTL("service:"+serviceID, "stopped", "warn")
				return
			case <-ticker.C:
				if statusFn == nil {
					_ = cr.AgentUpdateTTL("service:"+serviceID, "ok", "pass")
				} else {
					msg, state := statusFn()
					if state == "" {
						state = "pass"
					}
					_ = cr.AgentUpdateTTL("service:"+serviceID, msg, state)
				}
			}
		}
	}()
}
