package chat

import (
	"github.com/gorilla/websocket"
)

// Client represents a user session connected to the gateway.
// Suitable for msg_gateway in forwarding-only mode, without room logic.
// A single user may have multiple devices/connections, each maintained separately.

type Client struct {
	ConnID string          // Unique connection ID (unique within the local gateway)
	UserID string          // User ID (determined after authentication)
	WS     *websocket.Conn // WebSocket connection object
	Send   chan []byte     // Outbound message queue (consumed by a single writer goroutine)
}
