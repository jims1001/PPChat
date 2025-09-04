package message

import (
	pb "PProject/gen/message"
	"PProject/service/chat"

	msgcli "PProject/service/msg"
	util "PProject/tools"

	"github.com/golang/glog"
)

func HandlerTopicMessage(topic string, key, value []byte) error {
	msg, err := util.DecodeFrame(value)
	if err != nil {
		glog.Infof("topic key :%v Parse msg error: %s", topic, err)
		return err
	}
	/// 写数据库

	if msg.Type == pb.MessageFrameData_DATA {
		//msg.WsRelayBound <- msg
		err := msgcli.ReplayMsg(value)
		if err != nil {
			glog.Infof("topic key :%v Replay msg error: %s", topic, err)
			return err
		}

		deliverMsg := chat.BuildSendSuccessAckDeliver(msg.From, msg.GetPayload().ClientMsgId, msg.GetPayload().ServerMsgId, msg)
		deliverMsgData, err := util.EncodeFrame(deliverMsg)
		if err != nil {
			glog.Infof("BuildSendSuccessAckDeliver EncodeFrame topic key :%v Replay msg error: %s", topic, err)
		}
		err = msgcli.ReplayMsg(deliverMsgData)
		if err != nil {
			glog.Infof("BuildSendSuccessAckDeliver topic key :%v Replay msg error: %s", topic, err)
		}

	}
	
	// 下发给自己

	//glog.Infof("[TestTopic] key=%s, value=%s", key, value)
	return nil
}
