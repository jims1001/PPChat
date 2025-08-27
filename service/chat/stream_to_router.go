package chat

import (
	pb "PProject/gen/message"
	handler "PProject/service/chat/handlers"
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"log"
	"time"
)

type Server struct {
	gwID       string
	routerAddr string
	reg        *registry
	// below fields are used by ws_server for writing back to clients
	incoming    chan *pb.MessageFrame // frames from router destined to local users
	connection  chan *pb.MessageFrame // 只处理 连接的消息
	authPayload chan *pb.MessageFrameData

	connOutbound chan *pb.MessageFrameData
	authOutbound chan *WSConnectionMsg
	disp         *Dispatcher  // 处理器
	connMgr      *ConnManager // connection manager
}

type WSConnectionMsg struct {
	Frame *pb.MessageFrameData
	Conn  *WsConn
	Req   *pb.MessageFrameData
}

func SendFrameJSON(conn *websocket.Conn, frame *pb.MessageFrameData) error {
	if conn == nil {
		return fmt.Errorf("nil websocket conn")
	}

	// 转 JSON
	data, err := protojson.MarshalOptions{
		Indent:          "",   // 压缩格式
		UseEnumNumbers:  true, // 枚举输出数字（和你之前协议示例一致）
		EmitUnpopulated: false,
	}.Marshal(frame)
	if err != nil {
		return fmt.Errorf("marshal frame: %w", err)
	}

	// 发送
	return conn.WriteMessage(websocket.TextMessage, data)
}

func NewServer(gwID, routerAddr string, conn *ConnManager) (*Server, error) {
	return &Server{
		gwID:       gwID,
		routerAddr: routerAddr,
		reg:        newRegistry(),
		incoming:   make(chan *pb.MessageFrame, 4096),
		connMgr:    conn,
		disp:       NewDispatcher(),
	}, nil
}

func (s *Server) ConnMgr() *ConnManager {
	return s.connMgr
}

func (s *Server) registerHandlers() {
	ctx := &Context{S: s}
	s.disp.Register(handler.NewConnectHandler(ctx)) // CONN/REGISTER/UNREGISTER 相关
	s.disp.Register(handler.NewAuthHandler(ctx))    // AUTH 相关
	s.disp.Register(handler.NewPingHandler(ctx))
	// s.disp.Register(NewDeliverHandler(ctx)) // 如需要
}

func (s *Server) DispatchFrame(f *pb.MessageFrameData, conn *WsConn) error {
	return s.disp.Dispatch(&Context{S: s}, f, conn)
}

func (s *Server) RunToRouter() {
	retry := time.Second

	// 处理连接的消息发送
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.LoopConnect(ctx)
	s.LoopAuth(ctx)

	for {
		if err := s.loopRouter(); err != nil {
			log.Printf("router stream closed: %v, retry in %v", err, retry)
			time.Sleep(retry)
			if retry < 5*time.Second {
				retry *= 2
			}
		}
	}
}

// LoopConnect 处理连接信息
// loopConnect 消费 outbound 帧并发送到对应连接。
// 建议由上层传入 ctx（服务关闭时能优雅退出）。
func (s *Server) LoopConnect(ctx context.Context) {
	// 如果你有 WaitGroup：
	// s.wg.Add(1)
	go func() {
		// defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[loopConnect] panic recovered: %v", r)
			}
		}()

		outCh := s.ConnBound() // <-chan *pb.MessageFrameData

		marshaller := protojson.MarshalOptions{
			Indent:          "",
			UseEnumNumbers:  true,
			EmitUnpopulated: false,
		}

		for {
			select {
			case <-ctx.Done():
				log.Printf("[loopConnect] ctx done: %v", ctx.Err())
				return

			case msg, ok := <-outCh:
				if !ok {
					log.Printf("[loopConnect] outbound channel closed")
					return
				}
				if msg == nil {
					continue
				}
				connID := msg.GetConnId()
				if connID == "" {
					log.Printf("[loopConnect] missing conn_id, trace_id=%s type=%v", msg.GetTraceId(), msg.GetType())
					continue
				}

				ws, err := s.connMgr.GetUnAuthClient(msg.ConnId) // (*websocket.Conn, bool)
				if err != nil {
					log.Printf("[loopConnect] connMgr.GetUnAuthClient error: %v", err)
					continue
				}

				// 序列化（一次性）
				data, err := marshaller.Marshal(msg)
				if err != nil {
					log.Printf("[loopConnect] marshal frame failed: conn_id=%s err=%v", connID, err)
					continue
				}

				// 发送（带写超时）
				if err := writeJSONWithDeadline(ws.Conn, data, 5*time.Second); err != nil {
					log.Printf("[loopConnect] send failed: conn_id=%s err=%v", connID, err)
					// 发送失败：关闭并从管理器移除，防止死连接占用资源
					_ = ws.Conn.Close()
					s.connMgr.Remove(connID)
					continue
				}
			}
		}
	}()
}

func (s *Server) LoopAuth(ctx context.Context) {

	go func() {
		// defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[loopConnect] panic recovered: %v", r)
			}
		}()

		outCh := s.AuthBound() // <-chan *pb.MessageFrameData

		marshaller := protojson.MarshalOptions{
			Indent:          "  ", // 美化输出
			UseEnumNumbers:  true, // 枚举用数字
			EmitUnpopulated: true, // 建议调成 true，客户端好解析
		}

		for {
			select {
			case <-ctx.Done():
				log.Printf("[loopConnect] ctx done: %v", ctx.Err())
				return

			case msg, ok := <-outCh:
				if !ok {
					log.Printf("[loopConnect] outbound channel closed")
					return
				}
				if msg == nil {
					continue
				}
				connID := msg.Frame.GetConnId()
				if connID == "" {
					log.Printf("[loopConnect] missing conn_id, trace_id=%s type=%v", msg.Frame.GetTraceId(), msg.Frame.GetType())
					continue
				}

				ws, res := s.connMgr.Get(msg.Conn.UserId)
				if !res {
					log.Printf("[loopConnect] connMgr.GetUnAuthClient error: %v", res)
					continue
				}

				// 序列化（一次性）
				data, err := marshaller.Marshal(msg.Frame)
				if err != nil {
					log.Printf("[loopConnect] marshal frame failed: conn_id=%s err=%v", connID, err)
					continue
				}
				log.Printf("[loopConnect] send frame to data%s", string(data))

				// 发送（带写超时）
				if err := writeJSONWithDeadline(ws, data, 5*time.Second); err != nil {
					log.Printf("[loopConnect] send failed: conn_id=%s err=%v", connID, err)
					// 发送失败：关闭并从管理器移除，防止死连接占用资源
					_ = ws.Close()
					s.connMgr.Remove(connID)
					continue
				}
			}
		}
	}()
}

// 封装一个带写超时的发送，避免并发写/阻塞。
// 注意：gorilla/websocket 的 WriteMessage 不能并发调用，
// 如果上层可能多处写，请为每个连接做“单写协程 + 缓冲队列”的写泵模型。
func writeJSONWithDeadline(ws *websocket.Conn, jsonBytes []byte, d time.Duration) error {
	if ws == nil {
		return fmt.Errorf("nil websocket")
	}
	_ = ws.SetWriteDeadline(time.Now().Add(d))
	return ws.WriteMessage(websocket.TextMessage, jsonBytes)
}

func (s *Server) loopRouter() error {
	ctx := context.Background()
	cc, err := grpc.DialContext(ctx, s.routerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer func(cc *grpc.ClientConn) {
		err := cc.Close()
		if err != nil {

		}
	}(cc)
	client := pb.NewRouterClient(cc)
	stream, err := client.Gateway(ctx)
	if err != nil {
		return err
	}

	// announce ourselves (PING)
	_ = stream.Send(&pb.MessageFrame{Type: pb.MessageFrame_PING, GatewayId: s.gwID, Ts: time.Now().UnixMilli()})

	// reader: frames from router -> local incoming
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			f, err := stream.Recv()
			if err != nil {
				return
			}
			if f.GetType() == pb.MessageFrame_DELIVER {
				s.incoming <- f
			}
		}
	}()

	// writer: ws_server will push REGISTER/UNREGISTER/DATA frames via a channel
	for f := range s.Outbound() {
		f.GatewayId = s.gwID
		if err := stream.Send(f); err != nil {
			return err
		}
	}
	<-done
	return nil
}

// Outbound outbound returns a read-only channel that ws_server pushes into
func (s *Server) Outbound() chan *pb.MessageFrame { return WsOutbound }

func (s *Server) ConnBound() chan *pb.MessageFrameData {
	return WsConnection
}

// AuthBound 处理授权的消息 写端（供发送方使用）
func (s *Server) AuthBound() chan *WSConnectionMsg {
	return WsAuthChannel
}

// WsOutbound package-scope channel shared with ws_server.go for simplicity
var WsOutbound = make(chan *pb.MessageFrame, 8192)

// WsConnection 只需要处理连接
var WsConnection = make(chan *pb.MessageFrameData, 8192)

var WsAuthChannel = make(chan *WSConnectionMsg, 8192)
