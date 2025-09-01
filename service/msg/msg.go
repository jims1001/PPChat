package msg

import (
	"PProject/service/chat"
)

func init() {

}

func ReplayMsg(msg []byte) error {
	//return nil
	return chat.RelayMsg(msg)
}

func AcKMsg(msg []byte) error {
	return nil
}

func SendToUser(userId string, value []byte) error {
	return nil
}
