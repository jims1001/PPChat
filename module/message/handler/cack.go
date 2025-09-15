package handler

import (
	pb "PProject/gen/message"
	"PProject/logger"
	"PProject/service/chat"
	ka "PProject/service/kafka"

	"google.golang.org/protobuf/encoding/protojson"
)

type CAckHandler struct {
	ctx  *chat.ChatContext
	data chan *pb.MessageFrameData
}

func (h *CAckHandler) IsHandler() bool {
	return false // 消息方面的处理 是服务端主动回复
}

func NewCAckHandler(ctx *chat.ChatContext) chat.Handler { return &CAckHandler{ctx: ctx} }

func (h *CAckHandler) Type() pb.MessageFrameData_Type { return pb.MessageFrameData_CACK }

func (h *CAckHandler) Handle(_ *chat.ChatContext, f *pb.MessageFrameData, conn *chat.WsConn) error {
	to := f.To // 接收者
	// 判断接收者是否在线 如果不在线 就发松mq 落库 如果在线 看下 在那个节点， 找到那个节点 发送节点相关的topic
	logger.Infof("[WS] 接收到消息  MessageFrameData_CACK =%v toUser:%v ", f.From, to)

	topicKey := ka.SelectCAckTopicByUser(f.To, ka.GenCAckTopic())

	marshaller := protojson.MarshalOptions{
		Indent:          "",    // 美化输出
		UseEnumNumbers:  true,  // 枚举用数字
		EmitUnpopulated: false, // 建议调成 true，客户端好解析
	}

	data, err := marshaller.Marshal(f)
	if err != nil {
		logger.Errorf("[DataHandler] marshal err =%v ", err)
		return err
	}

	if h.ctx.S.MsgHandler != nil {

		err := h.ctx.S.MsgHandler(topicKey, f.To, data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *CAckHandler) Run() {

}
