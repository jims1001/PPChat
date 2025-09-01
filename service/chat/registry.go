package chat

import (
	"sync"
)

// each WebSocket
type conn struct {
	id        string
	snowId    string
	userId    string
	sessionId string
	send      chan []byte //

}

type Registry struct {
	mu     sync.RWMutex
	byUser map[string]map[string]*conn // user -> conn_id -> conn
	byConn map[string]*conn            // conn_id -> conn
}

func NewRegistry() *Registry {
	return &Registry{
		byUser: make(map[string]map[string]*conn),
		byConn: make(map[string]*conn),
	}
}

func (r *Registry) add(c *conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// user 索引
	m := r.byUser[c.sessionId]
	if m == nil {
		m = make(map[string]*conn)
		r.byUser[c.sessionId] = m
	}
	m[c.id] = c
	// conn 索引
	r.byConn[c.id] = c
}

func (r *Registry) remove(c *conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m := r.byUser[c.sessionId]; m != nil {
		delete(m, c.id)
		if len(m) == 0 {
			delete(r.byUser, c.snowId)
		}
	}
	delete(r.byConn, c.id)
}

func (r *Registry) listByUser(user string) []*conn {
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

func (r *Registry) getByConnID(connID string) *conn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byConn[connID]
}

// Optional: iterate through all connections (use sparingly, for debugging/statistics)
func (r *Registry) listAll() []*conn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*conn, 0, len(r.byConn))
	for _, c := range r.byConn {
		out = append(out, c)
	}
	return out
}
