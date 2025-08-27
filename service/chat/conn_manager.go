package chat

import (
	"PProject/tools/ids"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ===== 配置 =====

type ManagerConf struct {
	UnauthTTL   time.Duration    // 未授权连接的 TTL（如 60s）
	AuthTTL     time.Duration    // 已授权连接的 TTL（如 2h）
	SweepEvery  time.Duration    // 清理周期（如 10s）
	MaxPerUser  int              // 每用户最大连接数（<=0 不限制）
	EvictOldest bool             // 超限时是否淘汰最老连接（否则 Bind/Add 直接报错）
	Clock       func() time.Time // 可注入时钟（单测用）；nil => time.Now
}

func (c *ManagerConf) norm() {
	if c.Clock == nil {
		c.Clock = time.Now
	}
	if c.SweepEvery <= 0 {
		c.SweepEvery = 100 * time.Second
	}
	if c.UnauthTTL <= 0 {
		c.UnauthTTL = 300 * time.Second
	}
	if c.AuthTTL <= 0 {
		c.AuthTTL = 200 * time.Hour
	}
}

// ===== 数据结构 =====

type WsConn struct {
	SnowID     string
	UserId     string
	Authorized bool
	RId        string // redis 存储的key

	Conn   *websocket.Conn
	Remote net.Addr

	CreatedAt time.Time
	UpdatedAt time.Time
	SendChan  chan []byte // 每连接独立发送队列（业务二进制帧）

	TTL       time.Duration // 当前 TTL（随授权态切换）
	ExpireAt  time.Time     // 到期时间（过期由 sweeper 清理）
	Heartbeat time.Time     // 最近心跳时间
}

type ConnManager struct {
	mu     sync.RWMutex
	conns  map[string]*websocket.Conn    // 兼容旧版：userID -> 任意一条连接
	bySnow map[string]*WsConn            // 主索引：snowID -> wsConn
	byUser map[string]map[string]*WsConn // 辅助索引：userID -> (snowID -> wsConn)

	conf     ManagerConf
	stopOnce sync.Once
	stopCh   chan struct{}
	gwId     string // 节点ID
}

var wsConnPool = sync.Pool{
	New: func() any { return new(WsConn) },
}

// ===== 构造/关闭 =====

func NewConnManager(gwId string) *ConnManager {
	// 默认配置（与旧构造兼容）
	return NewConnManagerWithConf(ManagerConf{}, gwId)
}

func NewConnManagerWithConf(conf ManagerConf, gwId string) *ConnManager {
	conf.norm()
	m := &ConnManager{
		conns:  make(map[string]*websocket.Conn),
		bySnow: make(map[string]*WsConn),
		byUser: make(map[string]map[string]*WsConn),
		conf:   conf,
		gwId:   gwId,
		stopCh: make(chan struct{}),
	}
	go m.sweeper()
	return m
}

func (m *ConnManager) GwId() string {
	return m.gwId
}

func (m *ConnManager) Close() {
	m.stopOnce.Do(func() { close(m.stopCh) })
	// 关闭所有连接
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.bySnow {
		closeQuiet(x.Conn)
		x.Release()
	}

	m.conns = map[string]*websocket.Conn{}
	m.bySnow = map[string]*WsConn{}
	m.byUser = map[string]map[string]*WsConn{}

}

// ===== 兼容旧 API（Add/Get/Remove/Send）=====

// Add : 旧接口（只有 userID），内部自动生成 snowID；保留“单连接”兼容语义：
// 会把该用户既有连接清掉，只保留当前这条（如需多端，请使用 AddAuthorized / AddUnauth+BindUser）
func (m *ConnManager) Add(user string, sid string, conn *websocket.Conn) {
	if user == "" || conn == nil {
		return
	}
	now := m.conf.Clock()
	m.mu.Lock()
	defer m.mu.Unlock()

	// 兼容旧 map：单连接
	m.conns[user] = conn

	// 清理该用户所有新索引连接（保持旧行为）
	if mm := m.byUser[user]; mm != nil {
		for sid, w := range mm {
			delete(m.bySnow, sid)
			closeQuiet(w.Conn)
		}
		delete(m.byUser, user)
	}

	w := &WsConn{
		SnowID:     sid,
		UserId:     user,
		Authorized: true,
		Conn:       conn,
		Remote:     conn.RemoteAddr(),
		CreatedAt:  now,
		UpdatedAt:  now,
		TTL:        m.conf.AuthTTL,
		ExpireAt:   now.Add(m.conf.AuthTTL),
		Heartbeat:  now,
	}
	m.bySnow[sid] = w
	m.byUser[user] = map[string]*WsConn{sid: w}
}

// Get Get: 旧接口——返回该用户的一条连接（若有多条，返回任意一条）
func (m *ConnManager) Get(user string) (*websocket.Conn, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 旧索引优先
	if c, ok := m.conns[user]; ok && c != nil {
		return c, true
	}
	// 新索引兜底
	if mm := m.byUser[user]; mm != nil {
		for _, w := range mm {
			if w.Conn != nil {
				return w.Conn, true
			}
		}
	}
	return nil, false
}

// Remove : 旧接口——移除该用户所有连接
func (m *ConnManager) Remove(user string) {
	if user == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.conns, user)
	if mm := m.byUser[user]; mm != nil {
		for sid, w := range mm {
			delete(m.bySnow, sid)
			closeQuiet(w.Conn)
		}
		delete(m.byUser, user)
	}
}

// Send : 旧接口——向该用户**所有**连接广播；若无多端，则只发一条
func (m *ConnManager) Send(user string, data []byte) error {
	m.mu.RLock()
	mm := m.byUser[user]
	old := m.conns[user]
	m.mu.RUnlock()

	var lastErr error
	// 新索引多端广播
	for _, w := range mm {
		if err := writeBinary(w.Conn, data, 5); err != nil {
			lastErr = err
		}
	}
	// 兼容：若新索引没有，则发旧索引
	if len(mm) == 0 && old != nil {
		if err := writeBinary(old, data, 5); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// ===== 新能力：snowID / 未授权→授权 / 心跳 / TTL / 清理 / 最大连接数 / 挤下线 =====

// Release 完全释放，断开所有外部引用后 Put 回 Pool
func (c *WsConn) Release() {
	// 断引用，避免悬挂指针

	c.SnowID, c.UserId = "", ""
	c.TTL = 0
	c.ExpireAt = time.Time{}
	c.CreatedAt = time.Time{}
	c.UpdatedAt = time.Time{}
	c.Heartbeat = time.Time{}

	wsConnPool.Put(c)
}

func acquireWsConn(snowID string, conn *websocket.Conn, unauthTTL time.Duration, now time.Time) *WsConn {
	c := wsConnPool.Get().(*WsConn)
	*c = WsConn{} // 清零避免脏数据

	c.SnowID = snowID
	c.UserId = "" // 未授权
	c.Authorized = false
	c.Conn = conn
	if ra := conn.RemoteAddr(); ra != nil {
		c.Remote = conn.RemoteAddr()
	}

	// 如有发送队列/ctx，这里新建（可选）
	// c.SendQ = make(chan *pb.MessageFrameData, 256)
	// c.ctx, c.cancel = context.WithCancel(context.Background())

	c.CreatedAt = now
	c.UpdatedAt = now
	c.Heartbeat = now
	c.TTL = unauthTTL
	c.ExpireAt = now.Add(unauthTTL)
	return c
}

// AddUnauth : 新连接（未授权）登记；仅有 snowID
func (m *ConnManager) AddUnauth(snowID string, conn *websocket.Conn) (*WsConn, error) {
	if snowID == "" || conn == nil {
		return nil, errors.New("snowID/conn empty")
	}
	now := m.conf.Clock()
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.bySnow[snowID]; exists {
		return nil, errors.New("snowID exists")
	}

	wsConnection := acquireWsConn(snowID, conn, m.conf.UnauthTTL, now)
	m.bySnow[snowID] = wsConnection

	return wsConnection, nil
}

// GetUnAuthClient 获取没有授权的客户端
func (m *ConnManager) GetUnAuthClient(snowID string) (*WsConn, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 旧索引优先
	if c, ok := m.bySnow[snowID]; ok && c != nil {
		return c, nil
	}
	return nil, errors.New("snowID not found")
}

// BindUser : 将未授权 snowID 绑定到 user；会切到 AuthTTL，并执行“最大连接数/挤下线”策略
func (m *ConnManager) BindUser(snowID, user string) error {
	if snowID == "" || user == "" {
		return errors.New("snowID/user empty")
	}
	now := m.conf.Clock()
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.bySnow[snowID]
	if !ok || w.Conn == nil {
		return errors.New("snowID not found")
	}

	// 如果已绑定其他用户，从旧 user 索引移除
	if w.Authorized && w.UserId != "" && w.UserId != user {
		if mm := m.byUser[w.UserId]; mm != nil {
			delete(mm, w.SnowID)
			if len(mm) == 0 {
				delete(m.byUser, w.UserId)
			}
		}
	}

	// 进入新 user 索引前，先做“最大连接数/挤下线”
	if m.conf.MaxPerUser > 0 {
		m.ensureRoomForUserLocked(user)
	}

	// 挂到 user
	if m.byUser[user] == nil {
		m.byUser[user] = make(map[string]*WsConn)
	}
	m.byUser[user][w.SnowID] = w

	// 切授权态
	w.UserId = user
	w.Authorized = true
	w.TTL = m.conf.AuthTTL
	w.ExpireAt = now.Add(m.conf.AuthTTL)
	w.UpdatedAt = now
	w.Heartbeat = now

	// 兼容旧索引（无则补）
	if _, ex := m.conns[user]; !ex {
		m.conns[user] = w.Conn
	}
	return nil
}

// AddAuthorized : 已有 userID+snowID 的情况，直接登记为“已授权”；同样触发最大连接数策略
func (m *ConnManager) AddAuthorized(user, snowID string, conn *websocket.Conn) error {
	if user == "" || snowID == "" || conn == nil {
		return errors.New("user/snowID/conn empty")
	}
	now := m.conf.Clock()
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.bySnow[snowID]; exists {
		return errors.New("snowID exists")
	}

	// 先保证名额
	if m.conf.MaxPerUser > 0 {
		m.ensureRoomForUserLocked(user)
	}

	w := &WsConn{
		SnowID:     snowID,
		UserId:     user,
		Authorized: true,
		Conn:       conn,
		Remote:     conn.RemoteAddr(),
		CreatedAt:  now,
		UpdatedAt:  now,
		TTL:        m.conf.AuthTTL,
		ExpireAt:   now.Add(m.conf.AuthTTL),
		Heartbeat:  now,
	}
	m.bySnow[snowID] = w
	if m.byUser[user] == nil {
		m.byUser[user] = make(map[string]*WsConn)
	}
	m.byUser[user][snowID] = w

	if _, ex := m.conns[user]; !ex {
		m.conns[user] = conn
	}
	return nil
}

// Heartbeat : 刷新某条连接的心跳与到期时间（未授权/已授权都可调）
func (m *ConnManager) Heartbeat(snowID string) error {
	if snowID == "" {
		return errors.New("snowID empty")
	}
	now := m.conf.Clock()
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.bySnow[snowID]
	if !ok || w.Conn == nil {
		return errors.New("snowID not found")
	}
	w.Heartbeat = now
	w.ExpireAt = now.Add(w.TTL)
	w.UpdatedAt = now
	return nil
}

// AttachPongHandler : 绑定 gorilla/websocket 的 PongHandler，自动心跳续期
// 你在握手成功后即可调用：AttachPongHandler(conn, snowID)
func (m *ConnManager) AttachPongHandler(conn *websocket.Conn, snowID string) {
	if conn == nil || snowID == "" {
		return
	}
	conn.SetPongHandler(func(appData string) error {
		_ = m.Heartbeat(snowID) // 忽略错误：连接可能刚好被清理
		return nil
	})
}

// RemoveBySnow : 关闭并移除指定 snowID
func (m *ConnManager) RemoveBySnow(snowID string) {
	if snowID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.bySnow[snowID]
	if !ok {
		return
	}
	delete(m.bySnow, snowID)

	// 从用户索引移除
	if w.Authorized && w.UserId != "" {
		if mm := m.byUser[w.UserId]; mm != nil {
			delete(mm, snowID)
			if len(mm) == 0 {
				delete(m.byUser, w.UserId)
				// 维护旧索引：如果旧索引正好指向这条，尝试换成该用户剩余任意一条；否则删掉
				if old, ok := m.conns[w.UserId]; ok && old == w.Conn {
					// 找替代
					for _, rest := range mm {
						if rest.Conn != nil {
							m.conns[w.UserId] = rest.Conn
							goto DONE
						}
					}
					delete(m.conns, w.UserId)
				}
			} else {
				// 维护旧索引：若旧索引指向被删的连接，则换成任意剩余一条
				if old, ok := m.conns[w.UserId]; ok && old == w.Conn {
					for _, rest := range mm {
						if rest.Conn != nil {
							m.conns[w.UserId] = rest.Conn
							goto DONE
						}
					}
					delete(m.conns, w.UserId)
				}
			}
		}
	}
DONE:
	closeQuiet(w.Conn)
}

// KickAllUnauth : 踢出所有“未授权”连接
func (m *ConnManager) KickAllUnauth() int {
	n := 0
	m.mu.Lock()
	defer m.mu.Unlock()

	for sid, w := range m.bySnow {
		if !w.Authorized {
			delete(m.bySnow, sid)
			closeQuiet(w.Conn)
			n++
		}
	}
	return n
}

// SendOne : 按 snowID 发送
func (m *ConnManager) SendOne(snowID string, data []byte) error {
	m.mu.RLock()
	w, ok := m.bySnow[snowID]
	m.mu.RUnlock()
	if !ok || w.Conn == nil {
		return errors.New("snowID not found")
	}
	return writeBinary(w.Conn, data, 5)
}

// BroadcastUser : 向某用户所有连接发送
func (m *ConnManager) BroadcastUser(user string, data []byte) error {
	m.mu.RLock()
	mm := m.byUser[user]
	m.mu.RUnlock()

	var lastErr error
	for _, w := range mm {
		if err := writeBinary(w.Conn, data, 5); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// ListUserConns : 列出用户所有连接（snowID -> *websocket.Conn）
func (m *ConnManager) ListUserConns(user string) map[string]*websocket.Conn {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]*websocket.Conn)
	if mm := m.byUser[user]; mm != nil {
		for sid, w := range mm {
			if w.Conn != nil {
				out[sid] = w.Conn
			}
		}
	}
	return out
}

// ===== 清理协程 =====

func (m *ConnManager) sweeper() {
	t := time.NewTicker(m.conf.SweepEvery)
	defer t.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case now := <-t.C:
			m.sweepOnce(now)
		}
	}
}

func (m *ConnManager) sweepOnce(now time.Time) {
	var expired []*WsConn

	m.mu.Lock()
	for sid, w := range m.bySnow {
		if now.After(w.ExpireAt) {
			// 收集后统一关闭，避免持锁期间关闭 socket
			expired = append(expired, w)
			// 从主索引删除
			delete(m.bySnow, sid)
			// 从用户索引删除
			if w.Authorized && w.UserId != "" {
				if mm := m.byUser[w.UserId]; mm != nil {
					delete(mm, sid)
					if len(mm) == 0 {
						delete(m.byUser, w.UserId)
						// 维护旧索引
						if old, ok := m.conns[w.UserId]; ok && old == w.Conn {
							delete(m.conns, w.UserId)
						}
					} else {
						// 维护旧索引
						if old, ok := m.conns[w.UserId]; ok && old == w.Conn {
							for _, rest := range mm {
								if rest.Conn != nil {
									m.conns[w.UserId] = rest.Conn
									break
								}
							}
						}
					}
				}
			}
		}
	}
	m.mu.Unlock()

	for _, w := range expired {
		closeQuiet(w.Conn)
	}
}

// ===== 最大连接数/挤下线 =====

// 需要在持锁状态下调用（*_Locked）
func (m *ConnManager) ensureRoomForUserLocked(user string) {
	if m.conf.MaxPerUser <= 0 {
		return
	}
	mm := m.byUser[user]
	if len(mm) < m.conf.MaxPerUser {
		return
	}
	if !m.conf.EvictOldest {
		// 不挤下线，直接报错（通过上层逻辑返回给调用者）
		panic("exceeds MaxPerUser without eviction; call site should pre-check or handle error mode")
	}

	// 选择最老的一条淘汰（CreatedAt 更早）
	var oldest *WsConn
	for _, w := range mm {
		if oldest == nil || w.CreatedAt.Before(oldest.CreatedAt) {
			oldest = w
		}
	}
	if oldest != nil {
		// 从索引移除，让 sweeper 外关闭
		delete(mm, oldest.SnowID)
		delete(m.bySnow, oldest.SnowID)
		// 维护旧索引
		if old, ok := m.conns[user]; ok && old == oldest.Conn {
			// 尝试换成剩余任意一条
			for _, rest := range mm {
				if rest.Conn != nil {
					m.conns[user] = rest.Conn
					goto DONE
				}
			}
			delete(m.conns, user)
		}
	DONE:
		go closeQuiet(oldest.Conn) // 解锁后关闭
	}
}

// ===== 工具函数 =====

func writeBinary(conn *websocket.Conn, data []byte, deadlineSec int) error {
	if conn == nil {
		return errors.New("nil conn")
	}
	if err := conn.SetWriteDeadline(time.Now().Add(time.Duration(deadlineSec) * time.Second)); err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, data)
}

func closeQuiet(c *websocket.Conn) {
	if c != nil {
		_ = c.Close()
	}

}

func genSnowID() string {
	return ids.GenerateString()
}
