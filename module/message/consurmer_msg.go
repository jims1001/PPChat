package message

import (
	pb "PProject/gen/message"

	msgcli "PProject/service/msg"
	util "PProject/tools"

	"github.com/golang/glog"
)

func HandlerTopicMessage(topic string, key, value []byte) error {
	msg, err := util.ParseFrameJSON(value)
	if err != nil {
		glog.Infof("topic key :%v Parse msg error: %s", topic, err)
		return err
	}

	if msg.Type == pb.MessageFrameData_DATA {
		//msg.WsRelayBound <- msg
		err := msgcli.ReplayMsg(value)
		if err != nil {
			glog.Infof("topic key :%v Replay msg error: %s", topic, err)
			return err
		}
	}
	
	//glog.Infof("[TestTopic] key=%s, value=%s", key, value)
	return nil
}
