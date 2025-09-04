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
