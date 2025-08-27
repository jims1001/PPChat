package handlers

import (
	pb "PProject/gen/message"
	"PProject/service/chat"
)

type ConnectHandler struct{ ctx *chat.Context }

func NewConnectHandler(ctx *chat.Context) chat.Handler   { return &ConnectHandler{ctx: ctx} }
func (h *ConnectHandler) Type() pb.MessageFrameData_Type { return pb.MessageFrameData_CONN }

func (h *ConnectHandler) Handle(_ *chat.Context, f *pb.MessageFrameData, conn *chat.WsConn) error {
	// 连接阶段消息统一走 ConnBound 写回
	h.ctx.S.ConnBound() <- f
	return nil
}
