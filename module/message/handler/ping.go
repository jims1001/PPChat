package handler

import (
	pb "PProject/gen/message"
	"PProject/service/chat"
	online "PProject/service/storage"
	"context"
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

				gwID := h.ctx.S.ConnMgr().GwId()
				userID := rec.UserId
				currentSnowID := rec.SnowID

				// 心跳这类操作不用给 600s，这里给 2s 足够，防止阻塞
				ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)

				reauthorized := false

				// 1. 看这个用户现在有没有在线
				gatewayID, _ := online.GetManager().GetUserGateway(ctx, userID)
				// 2. 没有记录，就直接绑定（Authorize），不走 Connect
				if gatewayID == "" {
					logger.Infof("[PingHandler] user=%s not online, authorize directly...", userID)
					if _, err := online.GetManager().Authorize(ctx, userID, currentSnowID); err != nil {
						logger.Errorf("[PingHandler] direct authorize failed gw=%s user=%s snow=%s err=%v",
							gwID, userID, currentSnowID, err)
					} else {
						logger.Infof("[PingHandler] direct authorize success gw=%s user=%s snow=%s",
							gwID, userID, currentSnowID)
						reauthorized = true
					}
				} else if gatewayID != gwID {
					// 在线但在别的网关
					logger.Errorf("[PingHandler] user=%s online at another gw=%s, expected=%s",
						userID, gatewayID, gwID)
					// 这里看你业务要不要强制迁移：
					// if _, err := online.GetManager().Authorize(ctx, userID, currentSnowID); err == nil {
					//     reauthorized = true
					// }
				}

				// 3. 如果这轮没刚刚绑定成功，就做心跳续期
				if !reauthorized {
					if ok, err := online.GetManager().HeartbeatAuthorized(gwID, userID, currentSnowID); err != nil {
						logger.Infof("[PingHandler] heartbeat failed: %v", err)
					} else if !ok {
						// 心跳说这条记录无效，就再做一次直接授权
						logger.Infof("[PingHandler] heartbeat returned false, re-authorizing user=%s", userID)
						if _, err := online.GetManager().Authorize(ctx, userID, currentSnowID); err != nil {
							logger.Errorf("[PingHandler] authorize retry failed gw=%s user=%s err=%v", gwID, userID, err)
						}
					}
				}

				cancel()

				// 4. 最后发 ping
				_ = conn.Conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.Conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
					logger.Infof("[PingHandler] ping err snowID=%s user=%s err=%v", currentSnowID, userID, err)
					return
				}
			}
		}
	}(rec)

	return nil
}
