package chat

import (
	pb "PProject/gen/message"
	"PProject/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"time"
)

var upgraded = websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096, CheckOrigin: func(r *http.Request) bool { return true }}

func (s *Server) HandleWS(c *gin.Context) {
	user := c.Query("user")
	if user == "" {
		c.String(400, "missing user")
		return
	}
	ws, err := upgraded.Upgrade(c.Writer, c.Request, nil) // Note the variable name "upgrader"
	if err != nil {
		return
	}

	const (
		presenceTTL  = 90 * time.Second
		pongWait     = 75 * time.Second
		pingInterval = 25 * time.Second // Must be less than pongWait/3
		writeWait    = 5 * time.Second
	)

	connID := time.Now().Format("150405.000000")
	ci := &conn{id: connID, user: user}

	s.reg.add(ci)

	s.connMgr.Add(user, ws)

	_ = storage.PresenceOnline(user, s.gwID, presenceTTL)

	// Read goroutine: read-only, no writing
	ws.SetReadLimit(1 << 20)
	_ = ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(pongWait))
	})

	// Use a quit signal to coordinate cleanup
	done := make(chan struct{})

	// Write goroutine: the only writer (business messages + ping + close)
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer func() {
			ticker.Stop()
			_ = storage.PresenceOnline(user, s.gwID, presenceTTL) // Renew presence
			// Always send close and shut down in the write goroutine
			_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
			_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = ws.Close()
			close(done)
		}()

		for {
			select {
			case f := <-s.incoming:
				// ⚠️ This channel is global and consumed by multiple connections; temporary filtering will still "steal" other users’ messages
				if f.GetTo() != user {
					// Skip messages not intended for this user (but they have already been consumed) — it’s recommended to switch to a per-connection send queue (see note below)
					continue
				}
				_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteMessage(websocket.BinaryMessage, f.GetPayload()); err != nil {
					// Log the error for troubleshooting
					// log.Printf("ws write err(%s): %v", user, err)
					return
				}

			case <-ticker.C:
				_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
					// log.Printf("ws ping err(%s): %v", user, err)
					return
				}
			}
		}
	}()

	// Registration/registration messages are sent only to outbound (do not write to ws in defer)
	wsOutbound <- &pb.MessageFrame{Type: pb.MessageFrame_REGISTER, From: user}

	// Read loop: read-only, exit on error, write goroutine handles cleanup
	for {
		mt, data, err := ws.ReadMessage()
		if err != nil {
			// log.Printf("ws read err(%s): %T %v", user, err, err)
			break
		}
		if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
			continue
		}
		to := c.Query("to") // Demo: in production, put this into the application frame
		wsOutbound <- &pb.MessageFrame{
			Type:    pb.MessageFrame_DATA,
			From:    user,
			To:      to,
			Payload: data,
			Ts:      time.Now().UnixMilli(),
		}
	}

	// Exit: unregister and remove from registry, wait for write goroutine to close ws
	s.reg.remove(ci)

	// Kick out
	_ = storage.PresenceOffline(user)
	s.connMgr.Remove(user)
	wsOutbound <- &pb.MessageFrame{Type: pb.MessageFrame_UNREGISTER, From: user}
	<-done
}
