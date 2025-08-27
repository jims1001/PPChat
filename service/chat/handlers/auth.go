package handlers

import (
	pb "PProject/gen/message"
	"PProject/service/chat"
	online "PProject/service/storage"
	errors "PProject/tools/errs"
	"context"
	"log"
	"time"
)

type AuthHandler struct{ ctx *chat.Context }

func NewAuthHandler(ctx *chat.Context) chat.Handler   { return &AuthHandler{ctx: ctx} }
func (h *AuthHandler) Type() pb.MessageFrameData_Type { return pb.MessageFrameData_AUTH }
func (h *AuthHandler) Handle(_ *chat.Context, f *pb.MessageFrameData, conn *chat.WsConn) error {
	payload := f.GetPayload()
	ap, err := chat.ExtractAuthPayload(payload)
	if err != nil {
		log.Printf("[auth] extract payload err: %v", err)
		return nil
	}
	if f.GetConnId() == "" {
		log.Printf("[auth] skip, empty ConnId user=%s", ap.UserID)
		return nil
	}

	// ★ FIX：Authorize 第三参传 ConnId
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, aerr := online.GetManager().Authorize(ctx, ap.UserID, f.GetConnId())
	cancel()
	if aerr != nil && !aerr.Is(&errors.ErrorRecordIsExist) {
		log.Printf("[auth] authorize err user=%s conn=%s: %v", ap.UserID, f.GetConnId(), aerr)
		return nil
	}

	err = h.ctx.S.ConnMgr().BindUser(f.GetConnId(), ap.UserID)
	if err != nil {
		log.Printf("[auth] bind user err: %v", err)
	}

	ack := chat.BuildAuthAck(f)

	h.ctx.S.AuthBound() <- &chat.WSConnectionMsg{
		Frame: ack,
		Conn:  nil,
		Req:   f,
	}

	return nil
}
