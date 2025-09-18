package registry

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Instance struct {
	Service   string
	ID        string
	Address   string
	Port      int
	Metadata  map[string]string // zone/az/weight/nodeId/...
	Ephemeral bool
}

type RegisterOptions struct {
	TTL          time.Duration // 仅记录；不同实现按需使用
	HealthURL    string        // 仅 Consul HTTP 健康用；本示例走 TTL，不使用
	WarmupWeight int           // 启动预热权重（可选）
}

type Registry interface {
	Register(ctx context.Context, inst Instance, opt RegisterOptions) error
	Deregister(ctx context.Context, service, id string) error
	List(ctx context.Context, service string) ([]Instance, error)
	Watch(ctx context.Context, service string) (Watcher, error)
	Close() error
	UpdateTTL(checkID string, note string, status string) error // 新增
}

type Watcher interface {
	Next() ([]Instance, error) // 阻塞直到更新（或错误/超时）
	Stop() error
}

var ErrStopped = errors.New("watcher stopped")

// ---------------- 平滑加权轮询（SWRR） ----------------

type swItem struct {
	inst      Instance
	weight    int
	current   int
	effective bool
}

type SWRR struct {
	mu   sync.Mutex
	list []*swItem
}

func NewSWRR() *SWRR { return &SWRR{} }

func parseWeight(meta map[string]string) int {
	if meta == nil {
		return 1
	}
	wstr, ok := meta["weight"]
	if !ok || wstr == "" {
		return 1
	}
	n, sign := 0, 1
	for i, c := range wstr {
		if i == 0 && c == '-' {
			sign = -1
			continue
		}
		if c < '0' || c > '9' {
			return 1
		}
		n = n*10 + int(c-'0')
	}
	n *= sign
	if n <= 0 {
		return 1
	}
	return n
}

func (b *SWRR) Update(insts []Instance) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.list = b.list[:0]
	for _, in := range insts {
		b.list = append(b.list, &swItem{
			inst:      in,
			weight:    parseWeight(in.Metadata),
			current:   0,
			effective: true,
		})
	}
}

func (b *SWRR) Next() (Instance, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.list) == 0 {
		return Instance{}, false
	}
	var total int
	var best *swItem
	for _, it := range b.list {
		if !it.effective || it.weight <= 0 {
			continue
		}
		it.current += it.weight
		total += it.weight
		if best == nil || it.current > best.current {
			best = it
		}
	}
	if best == nil {
		return Instance{}, false
	}
	best.current -= total
	return best.inst, true
}
