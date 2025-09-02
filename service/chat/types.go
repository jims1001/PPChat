package chat

import (
	pb "PProject/gen/message"
)

type Handler interface {
	Type() pb.MessageFrameData_Type
	Run() // 添加运行时
	Handle(*ChatContext, *pb.MessageFrameData, *WsConn) error
	IsHandler() bool // 是否需要处理
}

type ChatContext struct {
	S *Server
}
