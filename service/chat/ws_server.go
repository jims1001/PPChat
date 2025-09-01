package chat

import (
	pb "PProject/gen/message"
	online "PProject/service/storage"
	errors "PProject/tools/errs"
	"context"
	"net"
	"net/http"
	"time"

	"github.com/emicklei/go-restful/v3/log"
	"github.com/gin-gonic/gin"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
)

var upgraded = websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096, CheckOrigin: func(r *http.Request) bool { return true }}

func (s *Server) HandleWSV2(c *gin.Context) {

	ws, err := upgraded.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// 常见：非 WebSocket 请求/握手失败
		return
	}
	defer func(ws *websocket.Conn) {
		err := ws.Close()
		if err != nil {
			glog.Infof("[HandleWSV2] close websocket error: %v", err)
		}
	}(ws)

	// 构造连接的消息

}

// HandleWS ===== WebSocket 处理（修正版） =====
func (s *Server) HandleWS(c *gin.Context) {

	ws, err := upgraded.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// 常见：非 WebSocket 请求/握手失败
		return
	}

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

	conf := online.OnlineConfig{
		NodeID:        s.gwID,
		TTL:           presenceTTL,
		ChannelName:   "online_changes",
		SnowflakeNode: 1,
		UseClusterTag: true,
		MaxSessions:   5,
		UseJSONValue:  true,
		Secret:        "hmac-secret",
		UseEXAT:       true,
		UnauthTTL:     30 * time.Second, // 如遇“未授权清理过快”，可临时调大验证
	}

	// Online 管理器（幂等）
	_, _ = online.InitManager(conf)

	// --- 注册“未授权连接” → 返回 sessionKey + snowID ---
	sessionKey, snowID, err := online.GetManager().Connect(c)
	if err != nil {
		log.Printf("Connect (unauth) failed: %v", err)
		_ = ws.Close()
		return
	}
	log.Printf("[WS] new unauth conn snowID=%s sessionKey=%s", snowID, sessionKey)

	// 交给连接管理器登记（未授权）
	rec, err := s.connMgr.AddUnauth(snowID, ws)
	if err != nil {
		log.Printf("ConnMgr.AddUnauth failed: %v", err)
		_ = ws.Close()
		return
	}
	rec.RId = sessionKey

	rec.SendChan = make(chan []byte, 256) // 每连接独立发送队列

	// --- 基本 Read 配置 ---
	ws.SetReadLimit(1 << 20)
	_ = ws.SetReadDeadline(time.Now().Add(readPongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(readPongWait))
	})

	// --- 写协程：唯一写者（业务 + ping + 优雅关闭） ---
	done := make(chan struct{})
	go func(rec *WsConn) {
		ticker := time.NewTicker(pingInterval)
		first := time.NewTimer(firstPingDelay)
		defer func() {
			ticker.Stop()
			first.Stop()

			// 下线 presence（在真正关闭之前）
			//_ = online.GetManager().Offline()

			// 统一由写协程发 Close 并关闭底层连接
			_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
			_ = ws.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = ws.Close()

			// 回收连接管理
			s.connMgr.RemoveBySnow(rec.SnowID)
			close(done)
			log.Printf("[WS] closed snowID=%s user=%s", rec.SnowID, rec.UserId)
		}()

		// 注册事件（非阻塞，以免卡住当前 handler）
		select {
		case WsConnection <- BuildConnectionAck(rec.SnowID, s.gwID, sessionKey, rec.SnowID):
		default:
			log.Printf("[WS] wsConnection ch full, drop ack snowID=%s", rec.SnowID)
		}
		select {
		case WsOutbound <- &pb.MessageFrame{Type: pb.MessageFrame_REGISTER, From: rec.UserId}:
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
				_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteMessage(websocket.BinaryMessage, payload); err != nil {
					log.Printf("[WS] write payload err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
					return
				}
				// 成功写业务后，续期在线

				_, s2, err := online.GetManager().Connect(c)
				if err != nil {
					return
				}
				_, err = s.connMgr.AddUnauth(s2, ws)
				if err != nil {
					return
				}

			case <-first.C: // 首次 ping
				_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
					log.Printf("[WS] first ping err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
					return
				}

			case <-ticker.C: // 常规 ping
				_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
					log.Printf("[WS] ping err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
					return
				}
			}
		}
	}(rec)

	// ---- 读循环：只读，不写；出错即退出（写协程收尾） ----
	for {
		mt, data, rerr := ws.ReadMessage()
		if rerr != nil {
			if websocket.IsCloseError(rerr,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseNoStatusReceived,
			) {
				log.Printf("[WS] peer closed snowID=%s err=%v", rec.SnowID, rerr)
			} else if ne, ok := rerr.(net.Error); ok && ne.Timeout() {
				log.Printf("[WS] read timeout snowID=%s err=%v", rec.SnowID, rerr)
			} else {
				log.Printf("[WS] read err snowID=%s err=%v", rec.SnowID, rerr)
			}
			break
		}
		if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
			continue
		}

		// 解析业务帧
		msg, perr := ParseFrameJSON(data)
		if perr != nil {
			// 只打印简短样本
			sample := data
			if len(sample) > 256 {
				sample = sample[:256]
			}
			log.Printf("[WS] ParseFrameJSON err snowID=%s err=%v sample=%q len=%d",
				rec.SnowID, perr, sample, len(data))
			continue
		}

		if msg.Type == pb.MessageFrameData_AUTH {
			// 提取授权负载（按你的协议）
			payload := msg.GetPayload()
			payData, aerr := ExtractAuthPayload(payload)
			if aerr != nil {
				log.Printf("[WS] ExtractAuthPayload err snowID=%s err=%v", rec.SnowID, aerr)
				continue
			}
			if msg.ConnId == "" {
				log.Printf("[WS] Authorize skip (empty ConnId) user=%s snowID=%s", payData.UserID, rec.SnowID)
				continue
			}

			// 授权：注意第三个参数使用 ConnId（不是 SessionId）
			ctx, cancel := context.WithTimeout(context.Background(), authTimeout)
			_, authErr := online.GetManager().Authorize(ctx, payData.UserID, msg.SessionId)
			cancel()
			if authErr != nil {
				if !authErr.Is(&errors.ErrorRecordIsExist) {
					log.Printf("[WS] Authorize err user=%s conn=%s snowID=%s err=%v",
						payData.UserID, msg.ConnId, rec.SnowID, authErr)
					continue
				}
			}
			// 绑定用户 → 续期在线 → 放宽读超时
			rec.UserId = payData.UserID
			err := s.connMgr.BindUser(rec.SnowID, rec.UserId)
			if err != nil {
				continue
			}

			authAckMsg := BuildAuthAck(msg)

			WsAuthChannel <- &WSConnectionMsg{
				Frame: authAckMsg,
				Conn:  rec,
				Req:   msg,
			}

			_ = ws.SetReadDeadline(time.Now().Add(readIdleAfterAuth))
			log.Printf("[WS] authorized user=%s conn=%s snowID=%s", rec.UserId, msg.ConnId, rec.SnowID)
		} else if msg.Type == pb.MessageFrameData_DATA {

			to := msg.To // 接收者
			// 判断接收者是否在线 如果不在线 就发松mq 落库 如果在线 看下 在那个节点， 找到那个节点 发送节点相关的topic
			glog.Info("[WS] 接收到消息  fromUser =%v toUser:%v ", msg.From, to)

			WsDataChannel <- &WSConnectionMsg{
				Frame: msg,
				Conn:  rec,
				Req:   msg,
			}
			log.Printf("[WS] 接收到消息is user=%s conn=%s snowID=%s", rec.UserId, msg.ConnId, rec.SnowID)
		}

	}

	// ---- 退出阶段：标记未授权/下线、广播 UNREGISTER、等待写协程收尾 ----
	{
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// 未授权下线（如果仍处于未授权）
		if rec.UserId != "" {
			_, _ = online.GetManager().Offline(ctx, rec.UserId, rec.SnowID, true, "offline")
		} else {
			_, _ = online.GetManager().OfflineUnauth(ctx, rec.SnowID, true, "offline")
		}

		// 已授权连接：如果你有对应 API，可在这里做 Offline(user, snowID, ...)
	}

	// 向全局广播 UNREGISTER（非阻塞）
	select {
	case WsOutbound <- &pb.MessageFrame{Type: pb.MessageFrame_UNREGISTER, From: rec.UserId}:
	default:
		log.Printf("[WS] wsOutbound ch full, drop UNREGISTER user=%s", rec.UserId)
	}

	<-done // 等写协程真正关闭 ws & 回收
}
