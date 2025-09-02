package handler

import (
	pb "PProject/gen/message"
	"PProject/service/chat"
	online "PProject/service/storage"
	errors "PProject/tools/errs"
	"context"
	"log"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
)

type AuthHandler struct {
	ctx    *chat.ChatContext
	data   chan *chat.WSConnectionMsg
	cancel context.CancelFunc
}

func (h *AuthHandler) Run() {

	// 不要用 defer cancel()，要不然 Run() 一返回就 cancel 了
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel // 存到 struct，留给 Stop/Close 用

	h.data = make(chan *chat.WSConnectionMsg, 8192)

	go func() {
		// defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[AuthHandler] panic recovered: %v", r)
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
				log.Printf("[AuthHandler] ctx done: %v", ctx.Err())
				return

			case msg, ok := <-h.data:
				if !ok {
					log.Printf("[AuthHandler] outbound channel closed")
					return
				}
				if msg == nil {
					continue
				}
				connID := msg.Frame.GetSessionId()
				if connID == "" {
					log.Printf("[AuthHandler] missing conn_id, trace_id=%s type=%v", msg.Frame.GetTraceId(), msg.Frame.GetType())
					continue
				}

				ws, res := h.ctx.S.ConnMgr().Get(msg.Conn.UserId)
				if !res {
					log.Printf("[AuthHandler] connMgr.GetUnAuthClient error: %v", res)
					continue
				}

				// 序列化（一次性）
				data, err := marshaller.Marshal(msg.Frame)
				if err != nil {
					log.Printf("[AuthHandler] marshal frame failed: conn_id=%s err=%v", connID, err)
					continue
				}
				log.Printf("[AuthHandler] send frame to data%s", string(data))

				// 发送（带写超时）
				if err := chat.WriteJSONWithDeadline(ws, data, 5*time.Second); err != nil {
					log.Printf("[AuthHandler] send failed: conn_id=%s err=%v", connID, err)
					// 发送失败：关闭并从管理器移除，防止死连接占用资源
					_ = ws.Close()
					h.ctx.S.ConnMgr().Remove(connID)
					continue
				}
			}
		}
	}()
}

func (h *AuthHandler) IsHandler() bool {
	return true
}

func NewAuthHandler(ctx *chat.ChatContext) chat.Handler { return &AuthHandler{ctx: ctx} }

func (h *AuthHandler) Type() pb.MessageFrameData_Type { return pb.MessageFrameData_AUTH }

func (h *AuthHandler) Handle(_ *chat.ChatContext, f *pb.MessageFrameData, conn *chat.WsConn) error {
	payload := f.GetPayload()
	ap, err := chat.ExtractAuthPayload(payload)
	if err != nil {
		log.Printf("[AuthHandler] extract payload err: %v", err)
		return nil
	}
	if f.GetConnId() == "" {
		log.Printf("[AuthHandler] skip, empty ConnId user=%s", ap.UserID)
		return nil
	}

	// ★ FIX：Authorize 第三参传 ConnId
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, aerr := online.GetManager().Authorize(ctx, ap.UserID, f.GetSessionId())
	cancel()
	if aerr != nil && !aerr.Is(&errors.ErrorRecordIsExist) {
		log.Printf("[AuthHandler] authorize err user=%s conn=%s: %v", ap.UserID, f.GetSessionId(), aerr)
		return nil
	}

	err = h.ctx.S.ConnMgr().BindUser(f.GetSessionId(), ap.UserID)
	if err != nil {
		log.Printf("[AuthHandler] bind user err: %v", err)
	}

	rec := h.ctx.S.ConnMgr().GetClient(conn.Conn)

	ack := chat.BuildAuthAck(f)

	h.data <- &chat.WSConnectionMsg{
		Frame: ack,
		Conn:  rec,
		Req:   f,
	}

	return nil
}
