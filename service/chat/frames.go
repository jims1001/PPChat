package chat

// ParseFrameJSON/ExtractAuthPayload 来自你原始实现（保留）
import (
	pb "PProject/gen/message"
	decode "PProject/tools/decode"
	errors "PProject/tools/errs"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

func ParseFrameJSON(raw []byte) (*pb.MessageFrameData, error) {
	frame := &pb.MessageFrameData{}
	um := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := um.Unmarshal(raw, frame); err != nil {
		return nil, fmt.Errorf("unmarshal frame failed: %w", err)
	}
	return frame, nil
}

// WriteJSONWithDeadline 封装一个带写超时的发送，避免并发写/阻塞。
// 注意：gorilla/websocket 的 WriteMessage 不能并发调用，
// 如果上层可能多处写，请为每个连接做“单写协程 + 缓冲队列”的写泵模型。
func WriteJSONWithDeadline(ws *websocket.Conn, jsonBytes []byte, d time.Duration) error {
	if ws == nil {
		return fmt.Errorf("nil websocket")
	}
	_ = ws.SetWriteDeadline(time.Now().Add(d))
	return ws.WriteMessage(websocket.TextMessage, jsonBytes)
}

type AuthPayload struct {
	Type      string   `json:"type"`
	Token     string   `json:"token"`
	TokenHash string   `json:"token_hash"`
	TS        int64    `json:"ts,omitempty"`
	Nonce     string   `json:"nonce,omitempty"`
	UserID    string   `json:"user_id,omitempty"`
	DeviceID  string   `json:"device_id,omitempty"`
	Scope     []string `json:"scope,omitempty"`
	Sig       string   `json:"sig,omitempty"`
}

func ExtractAuthPayload(msg *pb.MessageData) (*AuthPayload, error) {
	if msg == nil {
		return nil, errors.New("nil MessageData")
	}
	ce := msg.GetCustomElem()
	if ce == nil {
		return nil, errors.New("custom_elem is nil")
	}
	st := ce.GetData()
	if st == nil {
		return nil, errors.New("custom_elem.data (Struct) is nil")
	}
	return decode.DecodeStruct[AuthPayload](st)
}

// ---- 构造若干服务端回执 ----

func BuildConnectionAck(connID, gatewayID, sessionID, nodeID string) *pb.MessageFrameData {
	now := time.Now().UnixMilli()
	return &pb.MessageFrameData{
		Type:      pb.MessageFrameData_CONN,
		Ts:        now,
		GatewayId: gatewayID,
		ConnId:    connID,
		SessionId: sessionID,
		Meta:      map[string]string{"node_id": nodeID},
	}
}

func BuildSessionAck(toUserID, connID, gatewayID, sessionKey, nodeID string) *pb.MessageFrameData {
	now := time.Now().UnixMilli()
	st, _ := structpb.NewStruct(map[string]any{
		"type":        "session_ack",
		"session_key": sessionKey,
		"node_id":     nodeID,
		"ts":          now,
	})
	msg := &pb.MessageData{
		ContentType:      399,
		SessionType:      4,
		MsgFrom:          2,
		SenderPlatformId: 9,
		CreateTime:       now,
		CustomElem: &pb.CustomElem{
			Description: "session_ack",
			Extension:   "v1",
			Data:        st,
		},
	}
	return &pb.MessageFrameData{
		Type:        pb.MessageFrameData_DELIVER,
		From:        "auth_service",
		To:          toUserID,
		Ts:          now,
		GatewayId:   gatewayID,
		ConnId:      connID,
		AppId:       "your-app",
		Body:        &pb.MessageFrameData_Payload{Payload: msg},
		Qos:         pb.MessageFrameData_QOS_AT_LEAST_ONCE,
		Priority:    pb.MessageFrameData_PRIORITY_DEFAULT,
		AckRequired: false,
	}
}

func BuildAuthAck(req *pb.MessageFrameData) *pb.MessageFrameData {
	now := time.Now().UnixMilli()
	st, _ := structpb.NewStruct(map[string]any{
		"ok":              true,
		"user_id":         req.From,
		"session_id":      req.SessionId,
		"conn_id":         req.ConnId,
		"device_id":       req.DeviceId,
		"granted_scopes":  []any{"read", "write", "profile"},
		"token_expire_at": now + 3600*1000,
		"server_time":     now,
		"heartbeat": map[string]any{
			"ping_interval_ms": 25000,
			"pong_timeout_ms":  75000,
		},
		"policy": map[string]any{
			"max_sessions":         5,
			"kick_unauth_after_ms": 30000,
		},
	})

	return &pb.MessageFrameData{
		Type:       pb.MessageFrameData_AUTH,
		From:       req.To,
		To:         req.From,
		Ts:         now,
		GatewayId:  req.GatewayId,
		ConnId:     req.ConnId,
		TenantId:   req.TenantId,
		AppId:      req.AppId,
		Qos:        pb.MessageFrameData_QOS_AT_LEAST_ONCE,
		Priority:   pb.MessageFrameData_PRIORITY_DEFAULT,
		AckId:      req.GetAckId(),
		DedupId:    fmt.Sprintf("authack-%d", now),
		Nonce:      req.Nonce,
		ExpiresAt:  now + 3600*1000,
		SessionId:  req.SessionId,
		DeviceId:   req.DeviceId,
		Platform:   req.Platform,
		AppVersion: req.AppVersion,
		Locale:     req.Locale,
		Meta:       map[string]string{"ip": "203.0.113.10", "ua": "Chrome/139"},
		Body: &pb.MessageFrameData_Payload{
			Payload: &pb.MessageData{
				ClientMsgId:      req.GetPayload().ClientMsgId,
				ServerMsgId:      req.SessionId,
				CreateTime:       now,
				SendTime:         now,
				SessionType:      4,
				SendId:           "gateway_auth",
				RecvId:           req.From,
				MsgFrom:          req.GetPayload().MsgFrom,
				ContentType:      req.GetPayload().ContentType,
				SenderPlatformId: 99,
				SenderNickname:   "System",
				IsRead:           false,
				Status:           0,
				CustomElem: &pb.CustomElem{
					Description: "session_ack",
					Extension:   "v1",
					Data:        st,
				},
			},
		},
	}
}

// BuildSendSuccessAckDeliver 消息发送成功回执消息
func BuildSendSuccessAckDeliver(toUser string, clientMsgID string, serverMsgID string, req *pb.MessageFrameData) *pb.MessageFrameData {
	now := time.Now().UnixMilli()
	return &pb.MessageFrameData{
		Type:      pb.MessageFrameData_DELIVER, // 4：服务端主动下发
		From:      "im_server" + req.GatewayId,
		To:        toUser,
		Ts:        now,
		GatewayId: req.GatewayId,
		ConnId:    req.ConnId,
		SessionId: req.SessionId,
		TenantId:  req.TenantId,
		AppId:     req.AppId,
		Qos:       pb.MessageFrameData_QOS_AT_LEAST_ONCE,
		DedupId:   "deliver-" + serverMsgID,
		Body: &pb.MessageFrameData_Payload{
			Payload: &pb.MessageData{
				ClientMsgId:      clientMsgID, // 回显幂等键
				ServerMsgId:      serverMsgID, // 服务端最终 ID
				SendTime:         now,
				SessionType:      int32(pb.SessionType_SINGLE_CHAT),
				MsgFrom:          int32(pb.MsgFrom_SYSTEM),
				ContentType:      int32(pb.ContentType_MsgNOTIFICATION), // 401
				SenderPlatformId: int32(pb.PlatformID_API),
				SenderNickname:   "System",
				Status:           0,
				NotificationElem: &pb.NotificationElem{
					Detail: `{"ok":true,"msg":"Message delivered successfully"}`,
				},
			},
		},
	}
}

func BuildPing(connID, gatewayID, sessionID, nodeID string) *pb.MessageFrameData {
	now := time.Now().UnixMilli()
	return &pb.MessageFrameData{
		Type:      pb.MessageFrameData_PING, // 帧类型改为 PING
		Ts:        now,                      // 当前毫秒时间戳
		GatewayId: gatewayID,                // 网关节点 ID
		ConnId:    connID,                   // 当前连接 ID
		SessionId: sessionID,                // 会话 ID
		Meta: map[string]string{ // 附加信息
			"node_id":          nodeID,
			"ping_interval_ms": "25000", // 建议客户端心跳间隔
			"pong_timeout_ms":  "75000", // 建议超时时间
		},
	}
}
