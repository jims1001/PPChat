package chat

import (
	pb "PProject/gen/gateway"

	"context"
	"fmt"
	"time"
)

type MsgGatewayService struct {
	srv *Server
	pb.UnimplementedGatewayControlServer
	conn *ConnManager
}

func NewMsgGatewayService(s *Server, conn *ConnManager) *MsgGatewayService {
	return &MsgGatewayService{srv: s, conn: conn}
}

func (c *MsgGatewayService) Send(ctx context.Context, in *pb.SendMessageFrame) (*pb.SendReply, error) {
	if in.GetType() == 0 {
		in.Type = pb.SendMessageFrame_DATA
	}
	if in.GetTs() == 0 {
		in.Ts = time.Now().UnixMilli()
	}

	// if conn_id specified, deliver exact
	if cid := in.GetConnId(); cid != "" {
		if ok := c.enqueueByConnID(cid, in.GetPayload()); ok {
			return &pb.SendReply{Ok: true}, nil
		}
		return &pb.SendReply{Ok: false, Error: "conn_id not found"}, nil
	}

	fmt.Printf("get payload: %s\n", string(in.Payload))

	// send message
	err := c.conn.Send(in.To, []byte(string(in.Payload)+"1"))
	if err != nil {
		return nil, err
	}

	delivered := c.enqueueByUser(in.GetTo(), in.GetPayload())
	if delivered > 0 {
		return &pb.SendReply{Ok: true}, nil
	}

	return &pb.SendReply{Ok: false, Error: "user not found"}, nil
}

func (c *MsgGatewayService) enqueueByUser(user string, payload []byte) int {
	conns := c.srv.reg.listByUser(user)
	n := 0
	for _, xc := range conns {
		select {
		case xc.send <- payload:
			n++
		default:
		}
	}
	return n
}

func (c *MsgGatewayService) enqueueByConnID(connID string, payload []byte) bool {
	conns := c.srv.reg.listAll()
	for _, xc := range conns {
		if xc.id == connID {
			select {
			case xc.send <- payload:
				return true
			default:
				return false
			}
		}
	}
	return false
}
