package chat

import (
	pb "PProject/gen/message"
	online "PProject/service/storage"
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

	// 处理ping消息
	// 处理连接信息
	// 处理正常的消息
}

// HandleWS ===== WebSocket 处理（修正版） =====
func (s *Server) HandleWS(c *gin.Context) {

	ws, err := upgraded.Upgrade(c.Writer, c.Request, nil)
	defer func(ws *websocket.Conn) {
		err := ws.Close()
		if err != nil {
			glog.Infof("[HandleWSV2] close websocket error: %v", err)
			return
		}
	}(ws)

	if err != nil {
		// 常见：非 WebSocket 请求/握手失败
		return
	}

	// ---- 常量参数（建议值） ----
	const (
		authTimeout       = 2 * time.Second // 从 400ms 拉长，避免偶发超时
		readIdleAfterAuth = 2 * time.Minute
	)

	connectHandler := s.Disp().GetHandler(pb.MessageFrameData_CONN)
	if connectHandler != nil {
		err := connectHandler.Handle(&ChatContext{
			s,
		}, nil, &WsConn{
			Conn: ws,
		})
		if err != nil {
			glog.Errorf("[connectHandler] connect handler error: %v", err)
			return
		}
	}

	pingHandler := s.Disp().GetHandler(pb.MessageFrameData_PING)
	if pingHandler != nil {
		err := pingHandler.Handle(&ChatContext{s}, nil, &WsConn{Conn: ws})
		if err != nil {
			glog.Infof("[pingHandler] ping error: %v", err)
			return
		}
	}
	rec := s.ConnMgr().GetClient(ws)
	glog.Infof("[HandleWS] rec %v", rec)
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

		glog.Infof("[HandleWS] 接收到数据 snowID=%s msg=%v", rec.SnowID, msg)

		dataHandler := s.Disp().GetHandler(msg.Type)
		if dataHandler == nil {
			glog.Infof("[HandleWS] 读取消息数据 no handler for message type=%d", msg.Type)
			continue
		}

		if msg.Type == pb.MessageFrameData_AUTH {

			err := dataHandler.Handle(&ChatContext{S: s}, msg, &WsConn{Conn: ws})
			if err != nil {
				glog.Infof("[HandleWS] dataHandler  for message type=%d", msg.Type)
				continue
			}

			//// 提取授权负载（按你的协议）
			//payload := msg.GetPayload()
			//payData, aerr := ExtractAuthPayload(payload)
			//if aerr != nil {
			//	log.Printf("[WS] ExtractAuthPayload err snowID=%s err=%v", rec.SnowID, aerr)
			//	continue
			//}
			//if msg.ConnId == "" {
			//	log.Printf("[WS] Authorize skip (empty ConnId) user=%s snowID=%s", payData.UserID, rec.SnowID)
			//	continue
			//}
			//
			//// 授权：注意第三个参数使用 ConnId（不是 SessionId）
			//ctx, cancel := context.WithTimeout(context.Background(), authTimeout)
			//_, authErr := online.GetManager().Authorize(ctx, payData.UserID, msg.SessionId)
			//cancel()
			//if authErr != nil {
			//	if !authErr.Is(&errors.ErrorRecordIsExist) {
			//		log.Printf("[WS] Authorize err user=%s conn=%s snowID=%s err=%v",
			//			payData.UserID, msg.ConnId, rec.SnowID, authErr)
			//		continue
			//	}
			//}
			//// 绑定用户 → 续期在线 → 放宽读超时
			//rec.UserId = payData.UserID
			//err := s.connMgr.BindUser(rec.SnowID, rec.UserId)
			//if err != nil {
			//	continue
			//}
			//
			//authAckMsg := BuildAuthAck(msg)
			//
			//WsAuthChannel <- &WSConnectionMsg{
			//	Frame: authAckMsg,
			//	Conn:  rec,
			//	Req:   msg,
			//}
			//
			//_ = ws.SetReadDeadline(time.Now().Add(readIdleAfterAuth))
			//log.Printf("[WS] authorized user=%s conn=%s snowID=%s", rec.UserId, msg.ConnId, rec.SnowID)
		} else if msg.Type == pb.MessageFrameData_DATA {

			//to := msg.To // 接收者
			// 判断接收者是否在线 如果不在线 就发松mq 落库 如果在线 看下 在那个节点， 找到那个节点 发送节点相关的topic

			dataHandler := s.Disp().GetHandler(msg.Type)
			if dataHandler == nil {
				glog.Infof("[HandleWS] dataHandler for message type=%d", msg.Type)
				continue
			}

			err := dataHandler.Handle(&ChatContext{S: s}, msg, &WsConn{Conn: ws})
			if err != nil {
				glog.Infof("[HandleWS] dataHandler for message type=%d", msg.Type)
				continue
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
