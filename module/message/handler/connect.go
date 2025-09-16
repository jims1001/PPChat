package handler

import (
	pb "PProject/gen/message"

	"PProject/logger"
	"PProject/service/chat"
	online "PProject/service/storage"
	errors "PProject/tools/errs"
	"context"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
)

type ConnectHandler struct {
	ctx    *chat.ChatContext
	data   chan *chat.WSConnectionMsg
	cancel context.CancelFunc
}

func (h *ConnectHandler) IsHandler() bool {
	return false
}

func NewConnectHandler(ctx *chat.ChatContext) chat.Handler {

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
		UserIndexTTL:  2 * time.Minute,
		UnauthTTL:     30 * time.Second, // 如遇“未授权清理过快”，可临时调大验证

	}

	// Online 管理器（幂等）
	_, _ = online.InitManager(conf)

	return &ConnectHandler{ctx: ctx}
}
func (h *ConnectHandler) Type() pb.MessageFrameData_Type { return pb.MessageFrameData_CONN }

func (h *ConnectHandler) Handle(_ *chat.ChatContext, f *pb.MessageFrameData, conn *chat.WsConn) error {

	c, _ := context.WithTimeout(context.Background(), 50*time.Second)
	//defer cancel()
	sessionKey, snowID, err := online.GetManager().Connect(c)
	if err != nil {
		logger.Errorf("[ConnectHandler] Connect (unauth) failed: %v", err)
		_ = conn.Conn.Close()
		return &errors.ErrInternalServer
	}

	logger.Infof("[ConnectHandler] new unauth conn snowID=%s sessionKey=%s", snowID, sessionKey)

	// 交给连接管理器登记（未授权）
	rec, err := h.ctx.S.ConnMgr().AddUnauth(snowID, conn.Conn)
	if err != nil {
		logger.Errorf("[ConnectHandler] ConnMgr.AddUnauth failed: %v", err)
		_ = conn.Conn.Close()
		return &errors.ErrInternalServer
	}
	rec.RId = sessionKey
	rec.SendChan = make(chan []byte, 256)

	connectAck := chat.BuildConnectionAck(snowID, h.ctx.S.ConnMgr().GwId(), sessionKey, snowID)
	h.data <- &chat.WSConnectionMsg{Frame: connectAck, Conn: conn}
	return nil
}

func (h *ConnectHandler) Run() {

	h.data = make(chan *chat.WSConnectionMsg, 8192)

	go func() {

		// 不要用 defer cancel()，要不然 Run() 一返回就 cancel 了
		ctx, cancel := context.WithCancel(context.Background())
		h.cancel = cancel // 存到 struct，留给 Stop/Close 用

		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("[ConnectHandler] panic recovered: %v", r)
			}
			h.cancel()
		}()

		marshaller := protojson.MarshalOptions{
			Indent:          "",
			UseEnumNumbers:  true,
			EmitUnpopulated: false,
		}

		for {
			select {
			case <-ctx.Done():
				logger.Infof("[ConnectHandler] ctx done: %v", ctx.Err())
				return

			case msg, ok := <-h.data:
				if !ok {
					logger.Infof("[ConnectHandler] outbound channel closed")
					return
				}
				if msg == nil {
					continue
				}

				connID := msg.Frame.GetConnId()
				if connID == "" {
					logger.Infof("[ConnectHandler] missing conn_id, trace_id=%s type=%v",
						msg.Frame.GetTraceId(), msg.Frame.GetType())
					continue
				}

				ws, err := h.ctx.S.ConnMgr().GetUnAuthClient(msg.Frame.ConnId)
				if err != nil {
					logger.Infof("[ConnectHandler] connMgr.GetUnAuthClient error: %v", err)
					continue
				}

				// 序列化（一次性）
				data, err := marshaller.Marshal(msg.Frame)
				if err != nil {
					logger.Infof("[ConnectHandler] marshal frame failed: conn_id=%s err=%v", connID, err)
					continue
				}

				// 发送（带写超时）
				if err := chat.WriteJSONWithDeadline(ws.Conn, data, 5*time.Second); err != nil {
					logger.Infof("[loopConnect] send failed: conn_id=%s err=%v", connID, err)
					_ = ws.Conn.Close()
					h.ctx.S.ConnMgr().Remove(connID)
					continue
				}
			}
		}

	}()
}
