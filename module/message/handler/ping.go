package handler

import (
	pb "PProject/gen/message"
	"PProject/service/chat"
	online "PProject/service/storage"
	"time"

	"PProject/logger"

	"github.com/gorilla/websocket"
)

// ---- 常量参数（建议值） ----
const (
	presenceTTL       = 300 * time.Second
	readPongWait      = 75 * time.Second
	pingInterval      = 25 * time.Second
	writeWait         = 10 * time.Second // 拉长以排查写超时9
	firstPingDelay    = 5 * time.Second  // 首个 ping 延后，避免刚连上即写超时
	authTimeout       = 2 * time.Second  // 从 400ms 拉长，避免偶发超时
	readIdleAfterAuth = 2 * time.Minute
)

type PingHandler struct {
	ctx  *chat.ChatContext
	data chan *chat.WSConnectionMsg
}

func (h *PingHandler) Run() {
	h.data = make(chan *chat.WSConnectionMsg, 8192)
}

func (h *PingHandler) IsHandler() bool {
	return false
}

func NewPingHandler(ctx *chat.ChatContext) chat.Handler {
	return &PingHandler{ctx: ctx}
}

func (h *PingHandler) Type() pb.MessageFrameData_Type {
	return pb.MessageFrameData_PING
}

func (h *PingHandler) Handle(_ *chat.ChatContext, f *pb.MessageFrameData, conn *chat.WsConn) error {

	// --- 基本 Read 配置 ---
	conn.Conn.SetReadLimit(1 << 20)
	_ = conn.Conn.SetReadDeadline(time.Now().Add(readPongWait))
	conn.Conn.SetPongHandler(func(string) error {
		return conn.Conn.SetReadDeadline(time.Now().Add(readPongWait))
	})

	rec := h.ctx.S.ConnMgr().GetClient(conn.Conn)
	// --- 写协程：唯一写者（业务 + ping + 优雅关闭） ---
	go func(rec *chat.WsConn) {
		ticker := time.NewTicker(pingInterval)
		first := time.NewTimer(firstPingDelay)

		defer func() {
			ticker.Stop()
			first.Stop()

			// 下线 presence（在真正关闭之前）
			//_ = online.GetManager().Offline()

			// 统一由写协程发 Close 并关闭底层连接
			_ = conn.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			_ = conn.Conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = conn.Conn.Close()

			// 回收连接管理
			h.ctx.S.ConnMgr().RemoveBySnow(rec.SnowID)
			logger.Infof("[PingHandler] closed snowID=%s user=%s", rec.SnowID, rec.UserId)
		}()

		// 循环处理：优先业务帧，其次首个 ping，再常规 ping
		for {
			select {
			case payload, ok := <-rec.SendChan:
				if !ok {
					return
				}
				_ = conn.Conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.Conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
					logger.Errorf("[PingHandler] write payload err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
					return
				}
				// 成功写业务后，续期在线

			case <-first.C: // 首次 ping
				_ = conn.Conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.Conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
					logger.Errorf("[PingHandler] first ping err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
					return
				}

			case <-ticker.C: // 常规 ping
				_ = conn.Conn.SetWriteDeadline(time.Now().Add(writeWait))

				if ok, err := online.GetManager().HeartbeatAuthorized(h.ctx.S.ConnMgr().GwId(), rec.UserId, rec.SnowID); err != nil {
					logger.Infof("[PingHandler] renew after biz write failed: %v", err)
				} else if !ok {
					logger.Infof("[PingHandler] renew after biz write returned false")
				}

				//logger.Infof("ping expire is sucess")

				if err := conn.Conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
					logger.Infof("[PingHandler] ping err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
					return
				}
			}
		}
	}(rec)

	return nil
}
