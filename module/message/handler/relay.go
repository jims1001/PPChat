package handler

import (
	pb "PProject/gen/message"
	"PProject/logger"
	chat "PProject/service/chat"
	"context"
	"google.golang.org/protobuf/encoding/protojson"
	"time"
)

type RelayHandler struct {
	ctx  *chat.ChatContext
	data chan *pb.MessageFrameData
}

func (h *RelayHandler) IsHandler() bool {
	return false // 消息方面的处理 是服务端主动回复
}

func NewRelayHandler(ctx *chat.ChatContext) chat.Handler { return &RelayHandler{ctx: ctx} }

func (h *RelayHandler) Type() pb.MessageFrameData_Type { return pb.MessageFrameData_DELIVER }

func (h *RelayHandler) Handle(_ *chat.ChatContext, f *pb.MessageFrameData, conn *chat.WsConn) error {
	h.data <- f
	return nil
}

func (h *RelayHandler) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.data = make(chan *pb.MessageFrameData, 8192)
	go func() {
		// defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("[RelayHandler] panic recovered: %v", r)
			}
		}()

		outCh := h.data // <-chan *pb.MessageFrameData

		marshaller := protojson.MarshalOptions{
			Indent:          "",    // 美化输出
			UseEnumNumbers:  true,  // 枚举用数字
			EmitUnpopulated: false, // 建议调成 true，客户端好解析
		}

		for {
			select {
			case <-ctx.Done():
				logger.Infof("[RelayHandler 数据处理] ctx done: %v", ctx.Err())
				return

			case msg, ok := <-outCh:
				if !ok {
					logger.Infof("[RelayHandler 数据处理] 数据处理通道已经关闭")
					return
				}
				if msg == nil {
					continue
				}

				ws, res := h.ctx.S.ConnMgr().Get(msg.To)
				if !res {
					logger.Infof("[RelayHandler] 获取到有效的客户端   error: %v", res)
					continue
				}

				// 序列化（一次性）
				data, err := marshaller.Marshal(msg)
				if err != nil {
					logger.Errorf("[RelayHandler] 解析数据出错 failed: conn_id=%s err=%v", err)
					continue
				}

				// 发送（带写超时）
				if err := chat.WriteJSONWithDeadline(ws, data, 5*time.Second); err != nil {
					logger.Errorf("[RelayHandler] send failed: conn_id=%s err=%v", err)
					// 发送失败：关闭并从管理器移除，防止死连接占用资源
					_ = ws.Close()
					h.ctx.S.ConnMgr().Remove(msg.To)
					continue
				}
			}
		}
	}()

}
