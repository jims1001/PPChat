package chat

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ConnManager struct {
	mu    sync.RWMutex
	conns map[string]*websocket.Conn // userID -> websocket connection
}

func NewConnManager() *ConnManager {
	return &ConnManager{
		conns: make(map[string]*websocket.Conn),
	}
}

// Add Register a user and their connection
func (m *ConnManager) Add(user string, conn *websocket.Conn) {
	m.mu.Lock()
	m.conns[user] = conn
	m.mu.Unlock()
}

// Remove a user connection
func (m *ConnManager) Remove(user string) {
	m.mu.Lock()
	delete(m.conns, user)
	m.mu.Unlock()
}

// Send message to specific user
func (m *ConnManager) Send(user string, data []byte) error {
	m.mu.RLock()
	conn, ok := m.conns[user]
	m.mu.RUnlock()
	if !ok {
		return nil // or return error if not found
	}

	err := conn.SetWriteDeadline(nowPlus(5))
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, data)
}

// Utility for write deadline
func nowPlus(durSec int) time.Time {
	return time.Now().Add(time.Duration(durSec) * time.Second)
}
