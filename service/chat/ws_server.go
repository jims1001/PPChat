package chat

import (
	pb "PProject/gen/message"
	"PProject/service/storage"
	online "PProject/service/storage"
	decode "PProject/tools/decode"
	errors "PProject/tools/errs"
	"context"
	"google.golang.org/protobuf/types/known/structpb"
	"log"

	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"net/http"
	"time"
)

var upgraded = websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096, CheckOrigin: func(r *http.Request) bool { return true }}

// AuthPayload 是 custom_elem.data 里的授权 JSON 负载。
type AuthPayload struct {
	Type      string   `json:"type"`                // 固定 "auth"
	Token     string   `json:"token"`               // 明文 token / JWT
	TokenHash string   `json:"token_hash"`          // hex(sha256(token))
	TS        int64    `json:"ts,omitempty"`        // 客户端毫秒时间戳（可选）
	Nonce     string   `json:"nonce,omitempty"`     // 随机串（可选）
	UserID    string   `json:"user_id,omitempty"`   // 冗余（可选）
	DeviceID  string   `json:"device_id,omitempty"` // 冗余（可选）
	Scope     []string `json:"scope,omitempty"`     // 权限（可选）
	Sig       string   `json:"sig,omitempty"`       // 如有额外签名（可选）
}

func ParseFrameJSON(raw []byte) (*pb.MessageFrameData, error) {
	frame := &pb.MessageFrameData{}
	um := protojson.UnmarshalOptions{
		DiscardUnknown: true, // 忽略未知字段，增强兼容性
	}
	if err := um.Unmarshal(raw, frame); err != nil {
		return nil, fmt.Errorf("unmarshal frame failed: %w", err)
	}
	return frame, nil
}

func ExtractAuthPayload(msg *pb.MessageData) (*AuthPayload, error) {
	if msg == nil {
		return nil, errors.New("nil MessageData")
	}
	ce := msg.GetCustomElem()
	if ce == nil {
		return nil, errors.New("custom_elem is nil")
	}
	st := ce.GetData() // ★ Data 是 *struct.Struct
	if st == nil {
		return nil, errors.New("custom_elem.data (Struct) is nil")
	}
	payload, err := decode.DecodeStruct[AuthPayload](st)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func BuildAuthAckSystemEvent(req *pb.MessageFrameData, ok bool, code int, reason string, extras map[string]string) *pb.MessageFrameData {
	if req == nil {
		return nil
	}
	now := time.Now().UnixMilli()

	// 复制必要头并对调 from/to
	resp := &pb.MessageFrameData{
		Type:        pb.MessageFrameData_DELIVER, // 回执/下发
		From:        "auth_service",              // 你的服务名/系统ID
		To:          req.GetFrom(),
		Ts:          now,
		GatewayId:   req.GetGatewayId(),
		ConnId:      req.GetConnId(),
		TenantId:    req.GetTenantId(),
		AppId:       req.GetAppId(),
		Qos:         pb.MessageFrameData_QOS_AT_MOST_ONCE,
		Priority:    pb.MessageFrameData_PRIORITY_DEFAULT,
		AckRequired: false,              // 回执一般不再需要 ACK
		AckId:       req.GetAckId(),     // 透传请求的 ack_id，便于前端/网关匹配
		TraceId:     req.GetTraceId(),   // 透传 trace
		SessionId:   req.GetSessionId(), // 透传会话
		DeviceId:    req.GetDeviceId(),
		Platform:    "Server",
		AppVersion:  "server-1.0.0",
		Locale:      req.GetLocale(),
		// payload 可以为空（仅用 system_event 承载）
	}

	// 组织 system_event 数据
	data := map[string]string{
		"ok":    boolToStr(ok),
		"code":  intToStr(code),
		"from":  "auth",
		"scope": "", // 可根据实际写入
	}
	// 合并 extras
	for k, v := range extras {
		data[k] = v
	}

	resp.SystemEvent = &pb.SystemEvent{
		EventType: "auth_ack",
		Reason:    reason,
		Data:      data,
	}
	return resp
}

// -------- 2) 构造 payload.custom_elem 风格回执（结构化通知） --------

// BuildAuthAckPayload 构造一个带 payload 的回执：payload.content_type=CUSTOM，
// custom_elem.description="auth_ack"，data 为 JSON 对象（Struct）。
func BuildAuthAckPayload(req *pb.MessageFrameData, ok bool, code int, reason string, info map[string]any) *pb.MessageFrameData {
	if req == nil {
		return nil
	}
	now := time.Now().UnixMilli()

	// 自定义 data（对象形式）
	payloadData := map[string]any{
		"type":   "auth_ack",
		"ok":     ok,
		"code":   code,
		"reason": reason,
		"ts":     now,
		// 这里可以带上 token_hash / user_id / device_id 等
	}
	for k, v := range info {
		payloadData[k] = v
	}
	st, _ := structpb.NewStruct(payloadData)

	msg := &pb.MessageData{
		// 你也可以填更多上下文，例如 send_id/recv_id 等
		SessionType:      4,   // NOTIFICATION，按你的枚举
		MsgFrom:          2,   // SYSTEM
		ContentType:      399, // CUSTOM
		SenderPlatformId: 9,   // API/Server，按你的枚举
		CreateTime:       now,
		CustomElem: &pb.CustomElem{
			Description: "auth_ack",
			Extension:   "v1",
			Data:        st, // ★ 关键：Struct 对象
		},
		TraceId:   req.GetTraceId(),
		SessionId: req.GetSessionId(),
	}

	resp := &pb.MessageFrameData{
		Type:        pb.MessageFrameData_DELIVER,
		From:        "auth_service",
		To:          req.GetFrom(),
		Ts:          now,
		GatewayId:   req.GetGatewayId(),
		ConnId:      req.GetConnId(),
		TenantId:    req.GetTenantId(),
		AppId:       req.GetAppId(),
		Qos:         pb.MessageFrameData_QOS_AT_MOST_ONCE,
		Priority:    pb.MessageFrameData_PRIORITY_DEFAULT,
		AckRequired: false,
		AckId:       req.GetAckId(),
		TraceId:     req.GetTraceId(),
		SessionId:   req.GetSessionId(),
		DeviceId:    req.GetDeviceId(),
		Platform:    "Server",
		AppVersion:  "server-1.0.0",
		Locale:      req.GetLocale(),
		Body:        &pb.MessageFrameData_Payload{Payload: msg},
	}
	return resp
}

// -------- 3) 同时带 system_event 与 payload 的复合回执（可选） --------

// BuildAuthAckFull 同时设置 system_event 与 payload.custom_elem。
// 有些前端只订阅 system_event，有些需要收消息，这样都能覆盖。
func BuildAuthAckFull(req *pb.MessageFrameData, ok bool, code int, reason string, extras map[string]string, info map[string]any) *pb.MessageFrameData {
	resp := BuildAuthAckPayload(req, ok, code, reason, info)
	if resp == nil {
		return nil
	}
	// 叠加 system_event
	data := map[string]string{
		"ok":   boolToStr(ok),
		"code": intToStr(code),
	}
	for k, v := range extras {
		data[k] = v
	}
	resp.SystemEvent = &pb.SystemEvent{
		EventType: "auth_ack",
		Reason:    reason,
		Data:      data,
	}
	return resp
}

// BuildSessionAck 构造“连接成功”的返回消息
func BuildSessionAck(toUserID, connID, gatewayID, sessionKey, nodeID string) *pb.MessageFrameData {
	now := time.Now().UnixMilli()

	// 构造 custom_elem.data 对象
	data := map[string]any{
		"type":        "session_ack",
		"session_key": sessionKey,
		"node_id":     nodeID,
		"ts":          now,
	}
	st, _ := structpb.NewStruct(data)

	// 构造 payload
	msg := &pb.MessageData{
		ContentType:      399, // CUSTOM
		SessionType:      4,   // 通知/系统会话，按你的枚举
		MsgFrom:          2,   // SYSTEM
		SenderPlatformId: 9,   // SERVER
		CreateTime:       now,
		CustomElem: &pb.CustomElem{
			Description: "session_ack",
			Extension:   "v1",
			Data:        st,
		},
	}

	// 构造 Frame
	return &pb.MessageFrameData{
		Type:        pb.MessageFrameData_DELIVER, // 下发
		From:        "auth_service",              // 或者 gateway 节点名
		To:          toUserID,
		Ts:          now,
		GatewayId:   gatewayID,
		ConnId:      connID,
		AppId:       "your-app", // 可选
		Body:        &pb.MessageFrameData_Payload{Payload: msg},
		Qos:         pb.MessageFrameData_QOS_AT_LEAST_ONCE,
		Priority:    pb.MessageFrameData_PRIORITY_DEFAULT,
		AckRequired: false,
	}
}

func BuildConnectionAck(connID, gatewayID, sessionID, nodeID string) *pb.MessageFrameData {
	now := time.Now().UnixMilli()
	return &pb.MessageFrameData{
		Type:      pb.MessageFrameData_CONN, // 连接确认
		Ts:        now,
		GatewayId: gatewayID,
		ConnId:    connID,
		SessionId: sessionID, // ★ 直接返回给客户端
		Meta: map[string]string{ // ★ 可选扩展信息
			"node_id": nodeID,
		},
		// payload 可以为空
	}
}

// -------- 4) 小工具 --------

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
func intToStr(i int) string {
	return fmtInt(i)
}

// 为避免额外依赖，这里简单封装（或直接用 strconv.Itoa）
func fmtInt(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func (s *Server) HandleWS(c *gin.Context) {
	user := c.Query("user")
	if user == "" {
		c.String(400, "missing user")
		return
	}
	ws, err := upgraded.Upgrade(c.Writer, c.Request, nil) // Note the variable name "upgrader"
	if err != nil {
		return
	}

	const (
		presenceTTL  = 90 * time.Second
		pongWait     = 75 * time.Second
		pingInterval = 25 * time.Second // Must be less than pongWait/3
		writeWait    = 5 * time.Second
	)

	conf := online.OnlineConfig{
		NodeID:        s.gwID,
		TTL:           presenceTTL,
		ChannelName:   "online_changes",
		SnowflakeNode: 1,
		UseClusterTag: true,
		MaxSessions:   5,
		UseJSONValue:  true,
		Secret:        "hmac-secret",
		UseEXAT:       true,
		UnauthTTL:     30 * time.Second,
	}

	// 处理在线状态
	_, _ = online.InitManager(conf)
	sessionKey, snowID, err := online.GetManager().Connect(c)

	if err != nil {
		log.Printf("get session key failed: %v", err)
	}
	println("sessionKey :%v snowID:%v", sessionKey, snowID)

	wsConn, err := s.connMgr.AddUnauth(snowID, ws)
	if err != nil {
		log.Printf("add unauth key failed: %v", err)
	}

	//_ = storage.PresenceOnline(user, s.gwID, presenceTTL)

	// Read goroutine: read-only, no writing
	ws.SetReadLimit(1 << 20)
	_ = ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(pongWait))
	})

	wsConnection <- BuildConnectionAck(snowID, s.gwID, sessionKey, snowID)

	// Use a quit signal to coordinate cleanup
	done := make(chan struct{})

	// Write goroutine: the only writer (business messages + ping + close)
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer func() {
			ticker.Stop()
			_ = storage.PresenceOnline(user, s.gwID, presenceTTL) // Renew presence
			// Always send close and shut down in the write goroutine
			_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
			_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = ws.Close()
			close(done)
		}()

		for {
			select {
			case f := <-s.incoming:
				// ⚠️ This channel is global and consumed by multiple connections; temporary filtering will still "steal" other users’ messages
				if f.GetTo() != user {
					// Skip messages not intended for this user (but they have already been consumed) — it’s recommended to switch to a per-connection send queue (see note below)
					continue
				}
				_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteMessage(websocket.BinaryMessage, f.GetPayload()); err != nil {
					// Log the error for troubleshooting
					// log.Printf("ws write err(%s): %v", user, err)
					return
				}

			case <-ticker.C:
				_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
					// log.Printf("ws ping err(%s): %v", user, err)
					return
				}
			}
		}
	}()

	// Registration/registration messages are sent only to outbound (do not write to ws in defer)
	wsOutbound <- &pb.MessageFrame{Type: pb.MessageFrame_REGISTER, From: user}

	// Read loop: read-only, exit on error, write goroutine handles cleanup
	for {
		mt, data, err := ws.ReadMessage()
		if err != nil {
			log.Printf("ws read err(%v)", err)
			break
		}
		if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
			continue
		}

		msg, error2 := ParseFrameJSON(data)
		if error2 != nil {
			log.Printf("ParseFrameJSON error:%v", error2)
			continue
		}

		payData, error1 := ExtractAuthPayload(msg.GetPayload())
		if error1 != nil {
			log.Printf("ExtractAuthPayload error:%v", error1)
			continue
		}

		ctx, _ := context.WithTimeout(context.Background(), 20*time.Millisecond)
		_, err = online.GetManager().Authorize(ctx, payData.UserID, msg.ConnId)
		if err != nil {
			log.Printf("Authorize error:%v", err)
			continue
		}

		log.Printf("payloadf:%v", payData)

	}

	// Exit: unregister and remove from registry, wait for write goroutine to close ws

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel() // 一定要调用，释放资源

	suc, err := online.GetManager().OfflineUnauth(ctx, wsConn.SnowID, true, "offline")
	if err != nil {
		log.Printf("offline unauth err:%v", err)
	}

	if suc {
		log.Printf("offline auth success")
	}
	s.connMgr.Remove(user)
	wsOutbound <- &pb.MessageFrame{Type: pb.MessageFrame_UNREGISTER, From: user}
	<-done
}
