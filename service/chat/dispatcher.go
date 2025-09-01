package chat

import (
	pb "PProject/gen/message"
	"fmt"

	"github.com/golang/glog"
)

type Dispatcher struct {
	handlers map[pb.MessageFrameData_Type]Handler
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{handlers: make(map[pb.MessageFrameData_Type]Handler)}
}

func (d *Dispatcher) Register(h Handler) { d.handlers[h.Type()] = h }

func (d *Dispatcher) Dispatch(ctx *Context, f *pb.MessageFrameData, conn *WsConn) error {
	h, ok := d.handlers[f.GetType()]
	if !ok {
		return fmt.Errorf("no handler for type=%v", f.GetType())
	}
	return h.Handle(ctx, f, conn)
}

func (d *Dispatcher) GetHandler(types pb.MessageFrameData_Type) Handler {

	h, ok := d.handlers[types]
	if !ok {
		glog.Infof("no handler for type=%v", types)
		return nil
	}
	return h
}
