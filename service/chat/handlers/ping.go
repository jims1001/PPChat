package handlers

import (
	pb "PProject/gen/message"
	"PProject/service/chat"
	online "PProject/service/storage"
	"context"
	"time"

	"github.com/emicklei/go-restful/v3/log"
	"github.com/gorilla/websocket"
)

// ---- 常量参数（建议值） ----
const (
	pingInterval   = 25 * time.Second
	writeWait      = 10 * time.Second // 拉长以排查写超时
	firstPingDelay = 5 * time.Second  // 首个 ping 延后，避免刚连上即写超时

)

type PingHandler struct{ ctx *chat.Context }

func NewPingHandler(ctx *chat.Context) chat.Handler { return &PingHandler{ctx: ctx} }

func (h *PingHandler) Handle(_ *chat.Context, f *pb.MessageFrameData, conn *chat.WsConn) error {
	return nil
}

func (h *PingHandler) Type() pb.MessageFrameData_Type {
	return pb.MessageFrameData_PING
}

func startPing(rec *chat.WsConn, conn *chat.ConnManager, ctx context.Context) {

	done := make(chan struct{})
	go func(rec *chat.WsConn) {
		ticker := time.NewTicker(pingInterval)
		first := time.NewTimer(firstPingDelay)
		defer func() {
			ticker.Stop()
			first.Stop()

			// 下线 presence（在真正关闭之前）
			//_ = online.GetManager().Offline()

			// 统一由写协程发 Close 并关闭底层连接
			_ = rec.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			_ = rec.Conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = rec.Conn.Close()

			// 回收连接管理
			conn.RemoveBySnow(rec.SnowID)
			close(done)
			log.Printf("[WS] closed snowID=%s user=%s", rec.SnowID, rec.UserId)
		}()

		// 注册事件（非阻塞，以免卡住当前 handler）
		select {
		case chat.WsConnection <- chat.BuildConnectionAck(rec.SnowID, conn.GwId(), rec.RId, rec.SnowID):
		default:
			log.Printf("[WS] wsConnection ch full, drop ack snowID=%s", rec.SnowID)
		}
		select {
		case chat.WsOutbound <- &pb.MessageFrame{Type: pb.MessageFrame_REGISTER, From: rec.UserId}:
		default:
			log.Printf("[WS] wsOutbound ch full, drop REGISTER user=%s", rec.UserId)
		}

		// 循环处理：优先业务帧，其次首个 ping，再常规 ping
		for {
			select {
			case payload, ok := <-rec.SendChan:
				if !ok {
					return
				}
				_ = rec.Conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := rec.Conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
					log.Printf("[WS] write payload err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
					return
				}
				// 成功写业务后，续期在线

				_, s2, err := online.GetManager().Connect(ctx)
				if err != nil {
					return
				}
				_, err = conn.AddUnauth(s2, rec.Conn)
				if err != nil {
					return
				}

			case <-first.C: // 首次 ping
				_ = rec.Conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := rec.Conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
					log.Printf("[WS] first ping err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
					return
				}

			case <-ticker.C: // 常规 ping
				_ = rec.Conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := rec.Conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
					log.Printf("[WS] ping err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
					return
				}
			}
		}
	}(rec)
}
