package registry

import (
	"PProject/logger"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"
)

const (
	ttlNotePass = "pass"
)

type SelfNode struct {
	ID        string // 物理实例ID，如 "topic-node-01"
	Address   string // 本机对外地址，如 "127.0.0.1"
	Port      int    // 本机对外端口（gRPC/HTTP）
	HealthURL string // 可选：HTTP 健康检查，如 "http://127.0.0.1:8080/health"
}

type Filter struct {
	Zone    string            // 例: "az1"
	Require map[string]string // 必须包含的元数据键值
}

type serviceState struct {
	mu  sync.RWMutex
	all []Instance
	lb  *SWRR

	watching bool
	cancel   context.CancelFunc
}

// MethodInfo 保存方法注册的完整信息
type MethodInfo struct {
	ServiceName string            `json:"service_name"`
	ServiceID   string            `json:"service_id"`
	Meta        map[string]string `json:"meta,omitempty"`
	Opts        RegisterOptions   `json:"opts,omitempty"`
	Name        string            `json:"name"`
}

func (m *MethodInfo) ToJSONString() (string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type ServiceManager struct {
	reg          Registry
	statesMu     sync.Mutex
	states       map[string]*serviceState
	refreshTTL   time.Duration
	selfInst     *Instance
	selfOpts     RegisterOptions
	methodInfos  map[string]MethodInfo // method -> info
	methodPrefix string
}

func (m *ServiceManager) SetSelf(inst Instance, opts RegisterOptions, methodPrefix string) {
	m.selfInst = &inst
	m.selfOpts = opts
	m.methodPrefix = methodPrefix
	if m.methodInfos == nil {
		m.methodInfos = make(map[string]MethodInfo)
	}
}

// RegisterMethod 注册/更新一个“方法即服务名”的虚拟服务（幂等覆盖）
func (m *ServiceManager) RegisterMethod(ctx *context.Context, method string, meta map[string]string) error {
	if m.selfInst == nil {
		return fmt.Errorf("self instance not set; call SetSelf first")
	}
	if meta == nil {
		meta = map[string]string{}
	}
	meta["method"] = method

	svcName := m.methodPrefix + "-" + method // e.g. "chat.GetTopicService"
	svcID := m.selfInst.ID + "#" + method    // e.g. "node-01#GetTopicService"

	// 若之前记录了不同ID（一般是 selfID 变化），先清理旧的
	if old, ok := m.methodInfos[method]; ok && old.ServiceID != svcID {
		_ = m.reg.Deregister(*ctx, old.ServiceName, old.ServiceID)
	}

	inst := Instance{
		Service:  svcName,
		ID:       svcID,
		Address:  m.selfInst.Address,
		Port:     m.selfInst.Port,
		Metadata: meta,
	}

	err := m.RegisterSelf(*ctx, inst, m.selfOpts)
	if err != nil {
		return err
	}

	m.methodInfos[method] = MethodInfo{
		ServiceName: svcName,
		ServiceID:   svcID,
		Meta:        meta,
		Name:        method,
		Opts:        m.selfOpts,
	}

	if err := m.StartWatch(*ctx, svcName); err != nil {
		// 不中断整个流程，记录一下
		logger.Errorf("watch %s error: %v", svcName, err)
	}

	return nil
}

// UpdateMethodMetaReplace 覆盖式更新（同 ID 再注册 = 覆盖 meta）
func (m *ServiceManager) UpdateMethodMetaReplace(ctx context.Context, method string, meta map[string]string) error {
	if m.selfInst == nil {
		return fmt.Errorf("self instance not set")
	}
	if meta == nil {
		meta = map[string]string{}
	}
	meta["method"] = method

	svcName := "chat." + method
	svcID := m.selfInst.ID + "#" + method

	inst := Instance{
		Service:  svcName,
		ID:       svcID,
		Address:  m.selfInst.Address,
		Port:     m.selfInst.Port,
		Metadata: meta,
	}
	return m.reg.Register(ctx, inst, m.selfOpts) // 同 ID 覆盖
}

// UpdateMethodMetaPatch 可选：增量 patch
func (m *ServiceManager) UpdateMethodMetaPatch(ctx context.Context, method string, patch map[string]string) error {
	if m.selfInst == nil {
		return fmt.Errorf("self instance not set")
	}
	// 读一份当前 meta（如果你本地保存了，直接用；否则可传入完整 meta 覆盖）
	// 简化：直接覆盖
	return m.UpdateMethodMetaReplace(ctx, method, patch)
}

func (m *ServiceManager) UnregisterMethod(ctx context.Context, method string) error {
	if m.selfInst == nil {
		return fmt.Errorf("self instance not set")
	}
	svcName := "chat." + method
	svcID := m.selfInst.ID + "#" + method
	if err := m.reg.Deregister(ctx, svcName, svcID); err != nil {
		return err
	}
	delete(m.methodInfos, method)
	return nil
}

func New(reg Registry, refreshTTL time.Duration) *ServiceManager {
	if refreshTTL <= 0 {
		refreshTTL = 30 * time.Second
	}
	return &ServiceManager{
		reg:        reg,
		states:     make(map[string]*serviceState),
		refreshTTL: refreshTTL,
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

			logger.Errorf("watch start error: %v", err)
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
				logger.Errorf("watch next error: %v", err)
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
	logger.Infof("service=%s updated: %d instances", service, len(list))
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

// BootBlocking BootBlocking：注册 → TTL 上报 → 订阅（多个服务）→ 阻塞到 ctx.Done() → 反注册与关闭
func (m *ServiceManager) BootBlocking(
	ctx context.Context,
	self Instance,
	ttl time.Duration,
	watchServices ...string,
) error {
	// 1) 注册自己
	//if err := m.RegisterSelf(ctx, self, RegisterOptions{}); err != nil {
	//	return err
	//}
	// 2) TTL 主动上报（内部自起 goroutine，不会阻塞）
	m.StartTTLReport(ctx, 10*time.Second)

	// 3) 订阅（需要本地缓存哪些服务就传哪些）
	for _, svc := range watchServices {
		if err := m.StartWatch(ctx, svc); err != nil {
			// 不中断整个流程，记录一下
			logger.Errorf("watch %s error: %v", svc, err)
		}
	}

	// 4) 阻塞等待退出信号
	<-ctx.Done()

	// 5) 退出清理：反注册 + 关闭
	//_ = m.DeregisterSelf(context.Background(), self.Service, self.ID)
	_ = m.Close()

	return nil
}

// StartBackground StartBackground：跟 BootBlocking 一样的启动流程，但在后台一个 goroutine 里跑；本函数快速返回
func (m *ServiceManager) StartBackground(
	parent context.Context,
	self Instance,
	ttl time.Duration,
	watchServices ...string,
) {
	ctx, cancel := context.WithCancel(parent)
	// 如果你需要暴露 cancel：可以把 cancel 返回给调用方
	_ = cancel

	go func() {
		_ = m.BootBlocking(ctx, self, ttl, watchServices...)
	}()
}

// GetMeta ：返回 service（比如 "chat.GetChatServiceTopic"）所有实例的 Meta
// 结果每项至少包含：_instance_id、_address、_port，以及原 Meta 键值
func (m *ServiceManager) GetMeta(service string) []map[string]string {
	list := m.All(service)
	out := make([]map[string]string, 0, len(list))
	for _, it := range list {
		row := map[string]string{
			"_instance_id": it.ID,
			"_address":     it.Address,
			"_port":        fmt.Sprint(it.Port),
		}
		for k, v := range it.Metadata {
			row[k] = v
		}
		out = append(out, row)
	}
	return out
}

// GetMetaKey ：聚合某键（去重、按字典序排序）
func (m *ServiceManager) GetMetaKey(service, key string) []string {
	list := m.All(service)
	seen := map[string]struct{}{}
	for _, it := range list {
		if v, ok := it.Metadata[key]; ok && v != "" {
			seen[v] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

var (
	DefaultMgr *ServiceManager
	once       sync.Once
)

// InitDefault InitDefault: 初始化全局单例 Manager
func InitDefault(reg Registry, refreshTTL time.Duration) *ServiceManager {
	once.Do(func() {
		DefaultMgr = New(reg, refreshTTL)
	})
	return DefaultMgr
}

// Global : 获取全局单例 Manager
func Global() *ServiceManager {
	return DefaultMgr
}

func Pick(service string, f *Filter) (Instance, bool) {
	if DefaultMgr == nil {
		return Instance{}, false
	}
	return DefaultMgr.Pick(service, f)
}

func All(service string) []Instance {
	if DefaultMgr == nil {
		return nil
	}
	return DefaultMgr.All(service)
}

func (m *ServiceManager) StartTTLReport(ctx context.Context, ttl time.Duration) {
	if ttl <= 0 {
		logger.Infof("TTL disabled: ttl=%v", ttl)
		return
	}
	// 建议半个 TTL 上报一次，带一点抖动
	base := ttl / 2
	if base < time.Second {
		base = time.Second
	}

	// 并发保护：methodInfos 读锁
	var mu sync.RWMutex
	getIDs := func() []string {
		mu.RLock()
		defer mu.RUnlock()
		var ids []string
		for _, info := range m.methodInfos {
			if info.ServiceID != "" {
				ids = append(ids, info.ServiceID)
			}
		}
		return ids
	}
	// 让 RegisterMethod/UnregisterMethod 在更新 m.methodInfos 时也能安全
	//（如果你的 m.methodInfos 本来就有 mutex，这里可以不用局部 mu）

	go func() {
		tick := time.NewTicker(base)
		defer tick.Stop()

		for {
			select {
			case <-tick.C:
				// 抖动
				time.Sleep(time.Duration(rand.Int63n(int64(base / 4))))
				ids := getIDs()
				for _, id := range ids {
					// 约定：TTL checkID = "service:<ServiceID>"
					checkID := "service:" + id
					if err := m.reg.UpdateTTL(checkID, ttlNotePass, "passing"); err != nil {
						logger.Errorf("ttl update failed id=%s err=%v", id, err)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}
