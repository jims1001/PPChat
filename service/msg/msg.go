package msg

import (
	"PProject/service/chat"
)

func init() {

}

func ReplayMsg(msg []byte, connectId string) error {

	//return nil
	return chat.RelayMsg(msg, connectId)
}

//// DispatchMsg 分发消息
//func DispatchMsg(msg []byte, topicKey string) error {
//	if global.GlobalConfig.NodeType == global.NodeTypeMsgGateWay {
//
//
//		// 就直接发消息
//		return nil
//	} else {
//		//return nil
//		return chat.RelayMsg(msg)
//	}
//}
