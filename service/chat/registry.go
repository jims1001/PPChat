package chat

import (
	"sync"
)

// each WebSocket
type conn struct {
	id   string
	user string
	send chan []byte //

}

type registry struct {
	mu     sync.RWMutex
	byUser map[string]map[string]*conn // user -> conn_id -> conn
	byConn map[string]*conn            // conn_id -> conn
}

func newRegistry() *registry {
	return &registry{
		byUser: make(map[string]map[string]*conn),
		byConn: make(map[string]*conn),
	}
}

func (r *registry) add(c *conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// user 索引
	m := r.byUser[c.user]
	if m == nil {
		m = make(map[string]*conn)
		r.byUser[c.user] = m
	}
	m[c.id] = c
	// conn 索引
	r.byConn[c.id] = c
}

func (r *registry) remove(c *conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m := r.byUser[c.user]; m != nil {
		delete(m, c.id)
		if len(m) == 0 {
			delete(r.byUser, c.user)
		}
	}
	delete(r.byConn, c.id)
}

func (r *registry) listByUser(user string) []*conn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := r.byUser[user]
	if len(m) == 0 {
		return nil
	}
	out := make([]*conn, 0, len(m))
	for _, c := range m {
		out = append(out, c)
	}
	return out
}

func (r *registry) getByConnID(connID string) *conn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byConn[connID]
}

// Optional: iterate through all connections (use sparingly, for debugging/statistics)
func (r *registry) listAll() []*conn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*conn, 0, len(r.byConn))
	for _, c := range r.byConn {
		out = append(out, c)
	}
	return out
}
