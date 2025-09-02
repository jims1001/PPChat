package handler

import (
	pb "PProject/gen/message"
	chat "PProject/service/chat"
	"context"
	"time"

	"github.com/golang/glog"
	"google.golang.org/protobuf/encoding/protojson"
)

type AckHandler struct {
	ctx  *chat.ChatContext
	data chan *pb.MessageFrameData
}

func (h *AckHandler) IsHandler() bool {
	return false // 消息方面的处理 是服务端主动回复
}

func NewAckHandler(ctx *chat.ChatContext) chat.Handler { return &AckHandler{ctx: ctx} }

func (h *AckHandler) Type() pb.MessageFrameData_Type { return pb.MessageFrameData_ACK }

func (h *AckHandler) Handle(_ *chat.ChatContext, f *pb.MessageFrameData, conn *chat.WsConn) error {
	h.data <- f
	return nil
}

func (h *AckHandler) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.data = make(chan *pb.MessageFrameData, 8192)
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
				connID := msg.GetConnId()
				if connID == "" {
					glog.Infof("[AckHandler 数据处理] 没有获取到连接 conn_id, trace_id=%s type=%v", msg.GetTraceId(), msg.GetType())
					continue
				}

				ws, res := h.ctx.S.ConnMgr().Get(msg.To)
				if !res {
					glog.Infof("[AckHandler 数据处理] 获取到有效的客户端   error: %v", res)
					continue
				}
				ackMsg := chat.BuildSendSuccessAckDeliver(msg.To,
					msg.GetPayload().ClientMsgId, msg.GetPayload().ServerMsgId, msg)
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

}
