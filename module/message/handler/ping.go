package handler

import (
	pb "PProject/gen/message"
	"PProject/service/chat"
	online "PProject/service/storage"
	"context"
	"time"

	"github.com/golang/glog"
	"google.golang.org/protobuf/encoding/protojson"
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
	ctx  *chat.Context
	data chan *chat.WSConnectionMsg
}

func (h *PingHandler) Run() {
	h.data = make(chan *chat.WSConnectionMsg, 8192)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		// defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				glog.Infof("[loopConnect] panic recovered: %v", r)
			}
		}()

		marshaller := protojson.MarshalOptions{
			Indent:          "",    // 美化输出
			UseEnumNumbers:  true,  // 枚举用数字
			EmitUnpopulated: false, // 建议调成 true，客户端好解析
		}

		for {
			select {
			case <-ctx.Done():
				glog.Infof("[AckHandler 数据处理] ctx done: %v", ctx.Err())
				return

			case msg, ok := <-h.data:
				if !ok {
					glog.Infof("[AckHandler 数据处理] 数据处理通道已经关闭")
					return
				}
				if msg == nil {
					continue
				}
				connID := msg.Frame.GetConnId()
				if connID == "" {
					glog.Infof("[AckHandler 数据处理] 没有获取到连接 conn_id, trace_id=%s type=%v", msg.Frame.GetTraceId(), msg.Frame.GetType())
					continue
				}

				ws, res := h.ctx.S.ConnMgr().Get(msg.Frame.To)
				if !res {
					glog.Infof("[AckHandler 数据处理] 获取到有效的客户端   error: %v", res)
					continue
				}
				ackMsg := chat.BuildSendSuccessAckDeliver(msg.Frame.To,
					msg.Frame.GetPayload().ClientMsgId, msg.Frame.GetPayload().ServerMsgId, msg.Frame)
				// 序列化（一次性）
				data, err := marshaller.Marshal(ackMsg)
				if err != nil {
					glog.Infof("[AckHandler 数据处理] 解析数据出错 failed: conn_id=%s err=%v", connID, err)
					continue
				}

				// 发送（带写超时）
				if err := chat.WriteJSONWithDeadline(ws, data, 5*time.Second); err != nil {
					glog.Infof("[AckHandler ] send failed: conn_id=%s err=%v", connID, err)
					// 发送失败：关闭并从管理器移除，防止死连接占用资源
					_ = ws.Close()
					h.ctx.S.ConnMgr().Remove(connID)
					continue
				}
			}
		}
	}()

	//done := make(chan struct{})
	//rec := &chat.WsConn{SnowID: "1"}
	//
	//go func(rec *chat.WsConn) {
	//	ticker := time.NewTicker(pingInterval)
	//	first := time.NewTimer(firstPingDelay)
	//	defer func() {
	//		ticker.Stop()
	//		first.Stop()
	//
	//		// 下线 presence（在真正关闭之前）
	//		//_ = online.GetManager().Offline()
	//
	//		// 统一由写协程发 Close 并关闭底层连接
	//		_ = rec.Conn.SetWriteDeadline(time.Now().Add(writeWait))
	//		_ = rec.Conn.WriteMessage(websocket.CloseMessage,
	//			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	//		_ = rec.Conn.Close()
	//
	//		// 回收连接管理
	//		h.ctx.S.ConnMgr().RemoveBySnow(rec.SnowID)
	//		close(done)
	//		log.Printf("[WS] closed snowID=%s user=%s", rec.SnowID, rec.UserId)
	//	}()
	//
	//	// 注册事件（非阻塞，以免卡住当前 handler）
	//	select {
	//	case h.data <- chat.BuildConnectionAck(rec.SnowID, h.ctx.S.ConnMgr().GwId(), rec.RId, rec.SnowID):
	//	default:
	//		log.Printf("[WS] wsConnection ch full, drop ack snowID=%s", rec.SnowID)
	//	}
	//	select {
	//	case chat.WsOutbound <- &pb.MessageFrame{Type: pb.MessageFrame_REGISTER, From: rec.UserId}:
	//	default:
	//		log.Printf("[WS] wsOutbound ch full, drop REGISTER user=%s", rec.UserId)
	//	}
	//
	//	// 循环处理：优先业务帧，其次首个 ping，再常规 ping
	//	for {
	//		select {
	//		case payload, ok := <-rec.SendChan:
	//			if !ok {
	//				return
	//			}
	//			_ = rec.Conn.SetWriteDeadline(time.Now().Add(writeWait))
	//			if err := rec.Conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
	//				log.Printf("[WS] write payload err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
	//				return
	//			}
	//			// 成功写业务后，续期在线
	//
	//			_, s2, err := online.GetManager().Connect(ctx)
	//			if err != nil {
	//				return
	//			}
	//			_, err = h.ctx.S.ConnMgr().AddUnauth(s2, rec.Conn)
	//			if err != nil {
	//				return
	//			}
	//
	//		case <-first.C: // 首次 ping
	//			_ = rec.Conn.SetWriteDeadline(time.Now().Add(writeWait))
	//			if err := rec.Conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
	//				log.Printf("[WS] first ping err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
	//				return
	//			}
	//
	//		case <-ticker.C: // 常规 ping
	//			_ = rec.Conn.SetWriteDeadline(time.Now().Add(writeWait))
	//			if err := rec.Conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
	//				log.Printf("[WS] ping err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
	//				return
	//			}
	//		}
	//	}
	//}(rec)
}

func (h *PingHandler) IsHandler() bool {
	return false
}

func NewPingHandler(ctx *chat.Context) chat.Handler {

	conf := online.OnlineConfig{
		NodeID:        ctx.S.ConnMgr().GwId(),
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

	return &PingHandler{ctx: ctx}
}

func (h *PingHandler) Handle(_ *chat.Context, f *pb.MessageFrameData, conn *chat.WsConn) error {
	return nil
}

func (h *PingHandler) Type() pb.MessageFrameData_Type {
	return pb.MessageFrameData_PING
}
