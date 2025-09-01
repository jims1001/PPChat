package handler

import (
	pb "PProject/gen/message"
	"PProject/service/chat"
	"context"
	"time"

	"github.com/golang/glog"
	"google.golang.org/protobuf/encoding/protojson"
)

type ConnectHandler struct {
	ctx  *chat.Context
	data chan *pb.MessageFrameData
}

func (h *ConnectHandler) IsHandler() bool {
	return false
}

func NewConnectHandler(ctx *chat.Context) chat.Handler   { return &ConnectHandler{ctx: ctx} }
func (h *ConnectHandler) Type() pb.MessageFrameData_Type { return pb.MessageFrameData_CONN }

func (h *ConnectHandler) Handle(_ *chat.Context, f *pb.MessageFrameData, conn *chat.WsConn) error {
	// 连接阶段消息统一走 ConnBound 写回
	h.ctx.S.ConnBound() <- f
	return nil
}

func (h *ConnectHandler) Run() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.data = make(chan *pb.MessageFrameData, 8192)
	// 如果你有 WaitGroup：
	// s.wg.Add(1)
	go func() {
		// defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {

				glog.Infof("[ConnectHandler] panic recovered: %v", r)
			}
		}()

		marshaller := protojson.MarshalOptions{
			Indent:          "",
			UseEnumNumbers:  true,
			EmitUnpopulated: false,
		}

		for {
			select {
			case <-ctx.Done():
				glog.Infof("[ConnectHandler] ctx done: %v", ctx.Err())
				return

			case msg, ok := <-h.data:
				if !ok {
					glog.Infof("[ConnectHandler] outbound channel closed")
					return
				}
				if msg == nil {
					continue
				}
				connID := msg.GetConnId()
				if connID == "" {
					glog.Infof("[ConnectHandler] missing conn_id, trace_id=%s type=%v", msg.GetTraceId(), msg.GetType())
					continue
				}

				ws, err := h.ctx.S.ConnMgr().GetUnAuthClient(msg.ConnId) // (*websocket.Conn, bool)
				if err != nil {
					glog.Infof("[ConnectHandler] connMgr.GetUnAuthClient error: %v", err)
					continue
				}

				// 序列化（一次性）
				data, err := marshaller.Marshal(msg)
				if err != nil {
					glog.Infof("[ConnectHandler] marshal frame failed: conn_id=%s err=%v", connID, err)
					continue
				}

				// 发送（带写超时）
				if err := chat.WriteJSONWithDeadline(ws.Conn, data, 5*time.Second); err != nil {
					glog.Infof("[loopConnect] send failed: conn_id=%s err=%v", connID, err)
					// 发送失败：关闭并从管理器移除，防止死连接占用资源
					_ = ws.Conn.Close()
					h.ctx.S.ConnMgr().Remove(connID)
					continue
				}
			}
		}
	}()
}
