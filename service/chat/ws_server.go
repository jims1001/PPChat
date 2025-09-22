package chat

import (
	pb "PProject/gen/message"
	online "PProject/service/storage"
	"context"
	"net"
	"net/http"
	"time"

	"PProject/logger"

	"github.com/emicklei/go-restful/v3/log"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgraded = websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096, CheckOrigin: func(r *http.Request) bool { return true }}

// HandleWS ===== WebSocket 处理（修正版） =====
func (s *Server) HandleWS(c *gin.Context) {

	ws, err := upgraded.Upgrade(c.Writer, c.Request, nil)
	defer func(ws *websocket.Conn) {
		err := ws.Close()
		if err != nil {

			logger.Infof("[HandleWSV2] close websocket error: %v", err)
			return
		}
	}(ws)

	if err != nil {
		// 常见：非 WebSocket 请求/握手失败
		logger.Infof("[HandleWSV2] upgrade websocket error: %v", err)
		return
	}

	connectHandler := s.Disp().GetHandler(pb.MessageFrameData_CONN)
	if connectHandler != nil {
		err := connectHandler.Handle(&ChatContext{
			s,
		}, nil, &WsConn{
			Conn: ws,
		})
		if err != nil {
			logger.Infof("[connectHandler] connect handler error: %v", err)
			return
		}
	}

	pingHandler := s.Disp().GetHandler(pb.MessageFrameData_PING)
	if pingHandler != nil {
		err := pingHandler.Handle(&ChatContext{s}, nil, &WsConn{Conn: ws})
		if err != nil {
			logger.Infof("[pingHandler] ping error: %v", err)
			return
		}
	}
	rec := s.ConnMgr().GetClient(ws)
	logger.Infof("[HandleWS] rec %v", rec)
	done := make(chan struct{})
	// ---- 读循环：只读，不写；出错即退出（写协程收尾） ----
	for {
		mt, data, rerr := ws.ReadMessage()
		if rerr != nil {
			if websocket.IsCloseError(rerr,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseNoStatusReceived,
			) {
				logger.Infof("[WS] peer closed snowID=%s err=%v", rec.SnowID, rerr)
			} else if ne, ok := rerr.(net.Error); ok && ne.Timeout() {
				logger.Infof("[WS] read timeout snowID=%s err=%v", rec.SnowID, rerr)
			} else {
				logger.Infof("[WS] read err snowID=%s err=%v", rec.SnowID, rerr)
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

		logger.Infof("[HandleWS] 接收到数据 snowID=%s msg=%v", rec.SnowID, msg)

		dataHandler := s.Disp().GetHandler(msg.Type)
		if dataHandler == nil {
			logger.Infof("[HandleWS] 读取消息数据 no handler for message type=%d", msg.Type)
			continue
		}

		if msg.Type == pb.MessageFrameData_AUTH {

			err := dataHandler.Handle(&ChatContext{S: s}, msg, &WsConn{Conn: ws})
			if err != nil {
				logger.Infof("[HandleWS] dataHandler  for message type=%d", msg.Type)
				continue
			}

		} else if msg.Type == pb.MessageFrameData_DATA || msg.Type == pb.MessageFrameData_CACK {

			//to := msg.To // 接收者
			// 判断接收者是否在线 如果不在线 就发松mq 落库 如果在线 看下 在那个节点， 找到那个节点 发送节点相关的topic

			dataHandler := s.Disp().GetHandler(msg.Type)
			if dataHandler == nil {
				logger.Infof("[HandleWS] dataHandler for message type=%d", msg.Type)
				continue
			}

			// connectId 关联上 好进行处理
			msg.SessionId = rec.SnowID
			err := dataHandler.Handle(&ChatContext{S: s}, msg, &WsConn{Conn: ws})
			if err != nil {
				logger.Infof("[HandleWS] dataHandler for message type=%d", msg.Type)
				continue
			}

			logger.Infof("[WS] 接收到消息is user=%s conn=%s snowID=%s", rec.UserId, msg.ConnId, rec.SnowID)
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
		logger.Infof("[WS] wsOutbound ch full, drop UNREGISTER user=%s", rec.UserId)
	}

	<-done // 等写协程真正关闭 ws & 回收
}
