package chat

import (
	pb "PProject/gen/message"
	online "PProject/service/storage"
	decode "PProject/tools/decode"
	errors "PProject/tools/errs"
	"context"
	"fmt"
	"github.com/emicklei/go-restful/v3/log"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"net"
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

func BuildAuthAck(req *pb.MessageFrameData) *pb.MessageFrameData {
	now := time.Now().UnixMilli()

	// 构造 custom_elem.data 的 JSON
	data := map[string]any{
		"ok":              true,
		"user_id":         "user_10001",
		"session_id":      "748508987506704384",
		"conn_id":         "c-77f0d2",
		"device_id":       "aabbccddeeff00112233",
		"granted_scopes":  []any{"read", "write", "profile"},
		"token_expire_at": now + 3600*1000, // 1小时后过期
		"server_time":     now,
		"heartbeat": map[string]any{
			"ping_interval_ms": 25000,
			"pong_timeout_ms":  75000,
		},
		"policy": map[string]any{
			"max_sessions":         5,
			"kick_unauth_after_ms": 30000,
		},
	}

	st, err := structpb.NewStruct(data)
	if err != nil {
		log.Printf("BuildAuthAck stdata is error: %v ", err)
	}
	return &pb.MessageFrameData{
		Type:        pb.MessageFrameData_AUTH,
		From:        "gateway_auth",
		To:          "user_10001",
		Ts:          now,
		GatewayId:   "gw-1a",
		ConnId:      "c-77f0d2",
		TenantId:    "tenant_001",
		AppId:       "im_app",
		Qos:         pb.MessageFrameData_QOS_AT_LEAST_ONCE,
		Priority:    pb.MessageFrameData_PRIORITY_DEFAULT,
		AckRequired: false,
		AckId:       req.GetAckId(), // 回显客户端的 ack_id
		DedupId:     fmt.Sprintf("authack-%d", now),
		Nonce:       "srv-nonce-123",
		ExpiresAt:   now + 3600*1000,
		SessionId:   "748508987506704384",
		DeviceId:    "aabbccddeeff00112233",
		Platform:    "Web",
		AppVersion:  "1.0.3",
		Locale:      "zh-CN",
		Meta: map[string]string{
			"ip": "203.0.113.10",
			"ua": "Chrome/139",
		},

		Body: &pb.MessageFrameData_Payload{
			Payload: &pb.MessageData{
				ClientMsgId:      "cmid-0001",
				ServerMsgId:      "smid-authack-0001",
				CreateTime:       now,
				SendTime:         now,
				SessionType:      4, // NOTIFICATION
				SendId:           "gateway_auth",
				RecvId:           "user_10001",
				MsgFrom:          2,     // SYSTEM
				ContentType:      31001, // AUTH_ACK (自定义)
				SenderPlatformId: 99,
				SenderNickname:   "System", // 把用户信息 JSON 放这里
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

// HandleWS ===== WebSocket 处理（修正版） =====
func (s *Server) HandleWS(c *gin.Context) {
	user := c.Query("user")
	if user == "" {
		c.String(http.StatusBadRequest, "missing user")
		return
	}

	ws, err := upgraded.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// 常见：非 WebSocket 请求/握手失败
		return
	}

	// ---- 常量参数（建议值） ----
	const (
		presenceTTL       = 300 * time.Second
		readPongWait      = 75 * time.Second
		pingInterval      = 25 * time.Second
		writeWait         = 10 * time.Second // 拉长以排查写超时
		firstPingDelay    = 5 * time.Second  // 首个 ping 延后，避免刚连上即写超时
		authTimeout       = 2 * time.Second  // 从 400ms 拉长，避免偶发超时
		readIdleAfterAuth = 2 * time.Minute
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
		UnauthTTL:     30 * time.Second, // 如遇“未授权清理过快”，可临时调大验证
	}

	// Online 管理器（幂等）
	_, _ = online.InitManager(conf)

	// --- 注册“未授权连接” → 返回 sessionKey + snowID ---
	sessionKey, snowID, err := online.GetManager().Connect(c)
	if err != nil {
		log.Printf("Connect (unauth) failed: %v", err)
		_ = ws.Close()
		return
	}
	log.Printf("[WS] new unauth conn snowID=%s sessionKey=%s", snowID, sessionKey)

	// 交给连接管理器登记（未授权）
	rec, err := s.connMgr.AddUnauth(snowID, ws)
	if err != nil {
		log.Printf("ConnMgr.AddUnauth failed: %v", err)
		_ = ws.Close()
		return
	}

	rec.SendChan = make(chan []byte, 256) // 每连接独立发送队列

	// --- 基本 Read 配置 ---
	ws.SetReadLimit(1 << 20)
	_ = ws.SetReadDeadline(time.Now().Add(readPongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(readPongWait))
	})

	// --- 写协程：唯一写者（业务 + ping + 优雅关闭） ---
	done := make(chan struct{})
	go func(rec *WsConn) {
		ticker := time.NewTicker(pingInterval)
		first := time.NewTimer(firstPingDelay)
		defer func() {
			ticker.Stop()
			first.Stop()

			// 下线 presence（在真正关闭之前）
			//_ = online.GetManager().Offline()

			// 统一由写协程发 Close 并关闭底层连接
			_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
			_ = ws.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = ws.Close()

			// 回收连接管理
			s.connMgr.RemoveBySnow(rec.SnowID)
			close(done)
			log.Printf("[WS] closed snowID=%s user=%s", rec.SnowID, rec.UserId)
		}()

		// 注册事件（非阻塞，以免卡住当前 handler）
		select {
		case wsConnection <- BuildConnectionAck(rec.SnowID, s.gwID, sessionKey, rec.SnowID):
		default:
			log.Printf("[WS] wsConnection ch full, drop ack snowID=%s", rec.SnowID)
		}
		select {
		case wsOutbound <- &pb.MessageFrame{Type: pb.MessageFrame_REGISTER, From: rec.UserId}:
		default:
			log.Printf("[WS] wsOutbound ch full, drop REGISTER user=%s", rec.UserId)
		}

		// 循环处理：优先业务帧，其次首个 ping，再常规 ping
		for {
			select {
			case payload, ok := <-rec.SendChan:
				if !ok {
					return
				}
				_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteMessage(websocket.BinaryMessage, payload); err != nil {
					log.Printf("[WS] write payload err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
					return
				}
				// 成功写业务后，续期在线

				_, s2, err := online.GetManager().Connect(c)
				if err != nil {
					return
				}
				_, err = s.connMgr.AddUnauth(s2, ws)
				if err != nil {
					return
				}

			case <-first.C: // 首次 ping
				_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
					log.Printf("[WS] first ping err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
					return
				}

			case <-ticker.C: // 常规 ping
				_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(writeWait)); err != nil {
					log.Printf("[WS] ping err snowID=%s user=%s err=%v", rec.SnowID, rec.UserId, err)
					return
				}
			}
		}
	}(rec)

	// ---- 读循环：只读，不写；出错即退出（写协程收尾） ----
	for {
		mt, data, rerr := ws.ReadMessage()
		if rerr != nil {
			if websocket.IsCloseError(rerr,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseNoStatusReceived,
			) {
				log.Printf("[WS] peer closed snowID=%s err=%v", rec.SnowID, rerr)
			} else if ne, ok := rerr.(net.Error); ok && ne.Timeout() {
				log.Printf("[WS] read timeout snowID=%s err=%v", rec.SnowID, rerr)
			} else {
				log.Printf("[WS] read err snowID=%s err=%v", rec.SnowID, rerr)
			}
			break
		}
		if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
			continue
		}

		// 解析业务帧
		msg, perr := ParseFrameJSON(data)
		if perr != nil {
			// 只打印简短样本
			sample := data
			if len(sample) > 256 {
				sample = sample[:256]
			}
			log.Printf("[WS] ParseFrameJSON err snowID=%s err=%v sample=%q len=%d",
				rec.SnowID, perr, sample, len(data))
			continue
		}

		if msg.Type == pb.MessageFrameData_AUTH {
			// 提取授权负载（按你的协议）
			payload := msg.GetPayload()
			payData, aerr := ExtractAuthPayload(payload)
			if aerr != nil {
				log.Printf("[WS] ExtractAuthPayload err snowID=%s err=%v", rec.SnowID, aerr)
				continue
			}
			if msg.ConnId == "" {
				log.Printf("[WS] Authorize skip (empty ConnId) user=%s snowID=%s", payData.UserID, rec.SnowID)
				continue
			}

			// 授权：注意第三个参数使用 ConnId（不是 SessionId）
			ctx, cancel := context.WithTimeout(context.Background(), authTimeout)
			_, authErr := online.GetManager().Authorize(ctx, payData.UserID, msg.SessionId)
			cancel()
			if authErr != nil {
				if !authErr.Is(&errors.ErrorRecordIsExist) {
					log.Printf("[WS] Authorize err user=%s conn=%s snowID=%s err=%v",
						payData.UserID, msg.ConnId, rec.SnowID, authErr)
					continue
				}
			}
			// 绑定用户 → 续期在线 → 放宽读超时
			rec.UserId = payData.UserID
			err := s.connMgr.BindUser(rec.SnowID, rec.UserId)
			if err != nil {
				continue
			}

			authAckMsg := BuildAuthAck(msg)

			wsAuthChannel <- &WSConnectionMsg{
				Frame: authAckMsg,
				Conn:  rec,
				Req:   msg,
			}

			_ = ws.SetReadDeadline(time.Now().Add(readIdleAfterAuth))

			log.Printf("[WS] authorized user=%s conn=%s snowID=%s", rec.UserId, msg.ConnId, rec.SnowID)
		}

		// 如果该业务帧本身是上行消息（例如客户端发文本），按需路由给自己或别人：
		// 这里示例：回显给自己（真实场景请在你的路由层把消息投递到目标连接的 SendChan）
		// rec.SendChan <- msg.GetPayload().GetBizData()
	}

	// ---- 退出阶段：标记未授权/下线、广播 UNREGISTER、等待写协程收尾 ----
	{
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// 未授权下线（如果仍处于未授权）
		_, _ = online.GetManager().OfflineUnauth(ctx, rec.SnowID, true, "offline")
		// 已授权连接：如果你有对应 API，可在这里做 Offline(user, snowID, ...)
	}

	// 向全局广播 UNREGISTER（非阻塞）
	select {
	case wsOutbound <- &pb.MessageFrame{Type: pb.MessageFrame_UNREGISTER, From: rec.UserId}:
	default:
		log.Printf("[WS] wsOutbound ch full, drop UNREGISTER user=%s", rec.UserId)
	}

	<-done // 等写协程真正关闭 ws & 回收
}
