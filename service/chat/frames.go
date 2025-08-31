package chat

// ParseFrameJSON/ExtractAuthPayload 来自你原始实现（保留）
import (
	pb "PProject/gen/message"
	decode "PProject/tools/decode"
	errors "PProject/tools/errs"
	"fmt"
	"time"

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
