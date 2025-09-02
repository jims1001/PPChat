package handler

import (
	pb "PProject/gen/message"
	"PProject/service/chat"
	ka "PProject/service/kafka"
	"context"

	"github.com/golang/glog"
	"google.golang.org/protobuf/encoding/protojson"
)

type DataHandler struct {
	ctx    *chat.ChatContext
	data   chan *chat.WSConnectionMsg
	cancel context.CancelFunc
}

func (h *DataHandler) Run() {

	// 这个数据是需要转发的 所有可以不需要处理
}

func (h *DataHandler) IsHandler() bool {
	return false
}

func NewDataHandler(ctx *chat.ChatContext) chat.Handler { return &DataHandler{ctx: ctx} }

func (h *DataHandler) Type() pb.MessageFrameData_Type { return pb.MessageFrameData_DATA }

func (h *DataHandler) Handle(_ *chat.ChatContext, f *pb.MessageFrameData, conn *chat.WsConn) error {

	to := f.To // 接收者
	// 判断接收者是否在线 如果不在线 就发松mq 落库 如果在线 看下 在那个节点， 找到那个节点 发送节点相关的topic
	glog.Info("[WS] 接收到消息  fromUser =%v toUser:%v ", f.From, to)

	topicKey := ka.SelectTopicByUser(f.To, ka.GenTopics())

	marshaller := protojson.MarshalOptions{
		Indent:          "",    // 美化输出
		UseEnumNumbers:  true,  // 枚举用数字
		EmitUnpopulated: false, // 建议调成 true，客户端好解析
	}

	data, err := marshaller.Marshal(f)
	if err != nil {
		glog.Infof("[DataHandler] marshal err =%v ", err)
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
