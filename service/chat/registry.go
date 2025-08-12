package chat

import (
	"sync"
)

type conn struct {
	id   string
	user string
	// ws connection hidden in ws_server.go
}

type registry struct {
	mu     sync.RWMutex
	byUser map[string]map[string]*conn // user -> conn_id -> conn
}

func newRegistry() *registry { return &registry{byUser: make(map[string]map[string]*conn)} }

func (r *registry) add(c *conn) {
	r.mu.Lock()
	m := r.byUser[c.user]
	if m == nil {
		m = make(map[string]*conn)
		r.byUser[c.user] = m
	}
	m[c.id] = c
	r.mu.Unlock()
}

func (r *registry) remove(c *conn) {
	r.mu.Lock()
	if m := r.byUser[c.user]; m != nil {
		delete(m, c.id)
		if len(m) == 0 {
			delete(r.byUser, c.user)
		}
	}
	r.mu.Unlock()
}

func (r *registry) listByUser(user string) []*conn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := r.byUser[user]
	if len(m) == 0 {
		return nil
	}
	res := make([]*conn, 0, len(m))
	for _, c := range m {
		res = append(res, c)
	}
	return res
}
