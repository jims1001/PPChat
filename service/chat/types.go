package chat

import (
	pb "PProject/gen/message"
)

type Handler interface {
	Type() pb.MessageFrameData_Type
	Handle(*Context, *pb.MessageFrameData, *WsConn) error
}

type Context struct {
	S *Server
}
