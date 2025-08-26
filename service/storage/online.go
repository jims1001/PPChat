package storage

import (
	redis2 "PProject/service/storage/redis"
	errors "PProject/tools/errs"
	ids "PProject/tools/ids"
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strings"
	"time"
)

// ===== 配置 =====
type OnlineConfig struct {
	NodeID        string        // 节点ID（参与key命名）
	TTL           time.Duration // 已授权会话TTL
	ChannelName   string        // Pub/Sub频道名
	SnowflakeNode int64         // 雪花节点号（预留）
	UseClusterTag bool          // 是否使用Redis Cluster hash-tag对齐
	MaxSessions   int           // 单用户最大会话数（预留）
	UseJSONValue  bool          // 会话value是否存JSON（当前使用"1"）
	Secret        string        // 预留签名密钥
	UseEXAT       bool          // 使用EXAT/PEXPIREAT（更精准）
	UnauthTTL     time.Duration // 未授权宽限期（<=0表示禁用未授权阶段）
}

// ===== Lua 脚本 =====

// 原子绑定用户（未授权 -> 已授权）
// KEYS[1] = unauth index key (nidxu:<node> 或 nidxu:{<node>})
// KEYS[2] = user index key   (nidx:<node>:u:<userId> 或 nidx:{<node>:<userId>})
// ARGV[1] = unauth session key (n:<node>:id:<snow> 或 n:{<node>}:id:<snow>)
// ARGV[2] = authed session key (n:<node>:id:<snow>:u:<user> 或 n:{<node>:<user>}:id:<snow>)
// ARGV[3] = ttlSeconds
// ARGV[4] = nowUnix
// ARGV[5] = expireAtUnix
// ARGV[6] = useEXAT(0/1)
// 返回：1 绑定成功；0 未找到未授权键；-1 已存在已授权键（冲突）
const luaBindUser = `
local unauthZ = KEYS[1]
local userZ   = KEYS[2]
local kUnauth = ARGV[1]
local kAuth   = ARGV[2]
local ttl     = tonumber(ARGV[3])
local expAt   = tonumber(ARGV[5])
local useEXAT = tonumber(ARGV[6])

if redis.call("EXISTS", kAuth) == 1 then
  return -1
end
if redis.call("EXISTS", kUnauth) == 0 then
  redis.call("ZREM", unauthZ, kUnauth)
  return 0
end

redis.call("ZREM", unauthZ, kUnauth)
redis.call("DEL", kUnauth)

if useEXAT == 1 then
  redis.call("SET", kAuth, "1")
  redis.call("PEXPIREAT", kAuth, expAt * 1000)
else
  redis.call("SET", kAuth, "1", "EX", ttl)
end
redis.call("ZADD", userZ, expAt, kAuth)
redis.call("EXPIRE", userZ, ttl * 2)

return 1
`

// 清理未授权连接（超时踢出）
// KEYS[1] = unauth index key
// ARGV[1] = nowUnix
// 返回：被清理的未授权 session keys（数组）
const luaSweepUnauth = `
local unauthZ = KEYS[1]
local now     = tonumber(ARGV[1])
local victims = redis.call("ZRANGEBYSCORE", unauthZ, "-inf", now)
for _, v in ipairs(victims) do
  redis.call("ZREM", unauthZ, v)
  redis.call("DEL", v)
end
if redis.call("ZCARD", unauthZ) > 0 then
  redis.call("EXPIRE", unauthZ, 3600)
end
return victims
`

// —— 未授权“单会话离线”（删除会话键 + 从未授权索引移除） ——
// KEYS[1] = unauth index key
// ARGV[1] = unauth session key
// 返回：1=删掉了会话键；0=会话键不存在（幂等）
const luaOfflineUnauthOne = `
local unauthZ = KEYS[1]
local kUnauth = ARGV[1]
local existed = redis.call("DEL", kUnauth)
redis.call("ZREM", unauthZ, kUnauth)
return existed
`

// —— 已授权“单会话离线”（删除会话键 + 从用户索引移除） ——
// KEYS[1] = user index key
// ARGV[1] = authed session key
// 返回：1=删掉了会话键；0=会话键不存在（幂等）
const luaOfflineOne = `
local userZ = KEYS[1]
local kAuth = ARGV[1]
local existed = redis.call("DEL", kAuth)
redis.call("ZREM", userZ, kAuth)
return existed
`

// 某用户“全部会话”强制下线
// KEYS[1] = user index key
// 返回：被删除的会话键数组
const luaLogoutAll = `
local userZ = KEYS[1]
local members = redis.call("ZRANGE", userZ, 0, -1)
for _, k in ipairs(members) do
  redis.call("DEL", k)
end
redis.call("DEL", userZ)
return members
`

// 清理某用户“已过期”的会话（按 score<=now）
// KEYS[1] = user index key
// ARGV[1] = nowUnix
// 返回：被清理的会话键数组
const luaSweepAuthed = `
local userZ = KEYS[1]
local now   = tonumber(ARGV[1])

local victims = redis.call("ZRANGEBYSCORE", userZ, "-inf", now)
for _, v in ipairs(victims) do
  redis.call("ZREM", userZ, v)
  redis.call("DEL", v)
end

if redis.call("ZCARD", userZ) > 0 then
  redis.call("EXPIRE", userZ, 3600)
end
return victims
`

// ===== Store =====
type OnlineStore struct {
	conf OnlineConfig
	// 你原有的脚本（保留占位）
	luaBatch *redis.Script
	luaHB    *redis.Script
	// 新增脚本
	luaBind             *redis.Script
	luaSweep            *redis.Script
	luaOfflineUnauthOne *redis.Script
	luaOfflineOne       *redis.Script
	luaLogoutAll        *redis.Script
	luaSweepAuthed      *redis.Script
}

func newOnlineStore(conf OnlineConfig) *OnlineStore {
	m := &OnlineStore{conf: conf}
	m.initExtraScripts()
	return m
}

// 如果你已有 NewManager(...)，确保其中调用一次 m.initExtraScripts()
func (m *OnlineStore) initExtraScripts() {
	m.luaBind = redis.NewScript(luaBindUser)
	m.luaSweep = redis.NewScript(luaSweepUnauth)
	m.luaOfflineUnauthOne = redis.NewScript(luaOfflineUnauthOne)
	m.luaOfflineOne = redis.NewScript(luaOfflineOne)
	m.luaLogoutAll = redis.NewScript(luaLogoutAll)
	m.luaSweepAuthed = redis.NewScript(luaSweepAuthed)
}

// ===== Key 构造 =====

// 未授权会话键
// UseClusterTag=true: n:{<node>}:id:<snow>
// false:              n:<node>:id:<snow>
func (m *OnlineStore) unauthSessionKey(snowID string) string {
	if m.conf.UseClusterTag {
		return fmt.Sprintf("n:{%s}:id:%s", m.conf.NodeID, snowID)
	}
	return fmt.Sprintf("n:%s:id:%s", m.conf.NodeID, snowID)
}

// 未授权索引：放本节点所有未授权连接
// UseClusterTag=true: nidxu:{<node>}
// false:              nidxu:<node>
func (m *OnlineStore) unauthIndexKey() string {
	if m.conf.UseClusterTag {
		return fmt.Sprintf("nidxu:{%s}", m.conf.NodeID)
	}
	return fmt.Sprintf("nidxu:%s", m.conf.NodeID)
}

// 已授权会话键
// UseClusterTag=true: n:{<node>:<user>}:id:<snow>
// false:              n:<node>:id:<snow>:u:<user>
func (m *OnlineStore) sessionKey(userID, snowID string) string {
	if m.conf.UseClusterTag {
		return fmt.Sprintf("n:{%s:%s}:id:%s", m.conf.NodeID, userID, snowID)
	}
	return fmt.Sprintf("n:%s:id:%s:u:%s", m.conf.NodeID, snowID, userID)
}

// 用户索引ZSET（member=会话key, score=expireAtUnix）
// UseClusterTag=true: nidx:{<node>:<user>}
// false:              nidx:<node>:u:<user>
func (m *OnlineStore) userIndexKey(userID string) string {
	if m.conf.UseClusterTag {
		return fmt.Sprintf("nidx:{%s:%s}", m.conf.NodeID, userID)
	}
	return fmt.Sprintf("nidx:%s:u:%s", m.conf.NodeID, userID)
}

// ===== 未授权阶段 API =====

// Connect：创建一个未授权连接（仅连接未登录）
func (m *OnlineStore) Connect(ctx context.Context) (sessionKey, snowID string, err error) {
	if m.conf.UnauthTTL <= 0 {
		return "", "", errors.New("UnauthTTL not configured")
	}
	snowID = ids.GenerateString()
	sKey := m.unauthSessionKey(snowID)
	zKey := m.unauthIndexKey()

	now := time.Now()
	expAt := now.Add(m.conf.UnauthTTL).Unix()

	pipe := redis2.GetRedis().TxPipeline()
	if m.conf.UseEXAT {
		pipe.Set(ctx, sKey, "1", 0)
		pipe.PExpireAt(ctx, sKey, time.Unix(expAt, 0))
	} else {
		pipe.SetEx(ctx, sKey, "1", m.conf.UnauthTTL)
	}
	pipe.ZAdd(ctx, zKey, redis.Z{Score: float64(expAt), Member: sKey})
	pipe.Expire(ctx, zKey, m.conf.UnauthTTL*2)
	if _, err = pipe.Exec(ctx); err != nil {
		return "", "", err
	}
	return sKey, snowID, nil
}

// HeartbeatUnauth：未授权连接心跳（延长宽限）
func (m *OnlineStore) HeartbeatUnauth(ctx context.Context, snowID string) error {
	if m.conf.UnauthTTL <= 0 {
		return nil
	}
	sKey := m.unauthSessionKey(snowID)
	zKey := m.unauthIndexKey()

	now := time.Now()
	expAt := now.Add(m.conf.UnauthTTL).Unix()

	pipe := redis2.GetRedis().TxPipeline()
	if m.conf.UseEXAT {
		pipe.PExpireAt(ctx, sKey, time.Unix(expAt, 0))
	} else {
		pipe.Expire(ctx, sKey, m.conf.UnauthTTL)
	}
	pipe.ZAdd(ctx, zKey, redis.Z{Score: float64(expAt), Member: sKey})
	pipe.Expire(ctx, zKey, m.conf.UnauthTTL*2)
	_, err := pipe.Exec(ctx)
	return err
}

// SweepUnauth：清理所有超时未授权连接，并选择性广播“踢出”
func (m *OnlineStore) SweepUnauth(ctx context.Context, publish bool) ([]string, error) {
	if m.conf.UnauthTTL <= 0 {
		return nil, nil
	}
	zUnauth := m.unauthIndexKey()
	now := time.Now().Unix()
	victims, err := m.luaSweep.Run(ctx, redis2.GetRedis(), []string{zUnauth}, now).StringSlice()
	if err != nil {
		return nil, err
	}
	if publish && len(victims) > 0 {
		// 事件：UNAUTH_KICK:n:<node>:id:<snow>,...
		payload := "UNAUTH_KICK:" + strings.Join(victims, ",")
		_ = redis2.GetRedis().Publish(ctx, m.conf.ChannelName, payload).Err()
	}
	return victims, nil
}

// OfflineUnauth：未授权单会话离线（幂等）。可选广播。
func (m *OnlineStore) OfflineUnauth(ctx context.Context, snowID string, publish bool, reason string) (bool, error) {
	if m.conf.UnauthTTL <= 0 {
		// 未开启未授权阶段，直接视为无事发生
		return false, nil
	}
	kUnauth := m.unauthSessionKey(snowID)
	zUnauth := m.unauthIndexKey()

	rc, err := m.luaOfflineUnauthOne.Run(ctx, redis2.GetRedis(),
		[]string{zUnauth},
		kUnauth,
	).Int64()
	if err != nil {
		return false, err
	}
	ok := rc == 1
	if publish && ok {
		// 事件：UNAUTH_OFFLINE:<key>:<reason>
		msg := fmt.Sprintf("UNAUTH_OFFLINE:%s:%s", kUnauth, reason)
		_ = redis2.GetRedis().Publish(ctx, m.conf.ChannelName, msg).Err()
	}
	return ok, nil
}

// Authorize：把“未授权连接”原子绑定成“已授权会话”
func (m *OnlineStore) Authorize(ctx context.Context, userID, snowID string) (bool, error) {
	if m.conf.UnauthTTL <= 0 {
		return false, errors.New("unauth stage disabled")
	}
	kUnauth := m.unauthSessionKey(snowID)
	kAuth := m.sessionKey(userID, snowID)
	zUnauth := m.unauthIndexKey()
	zUser := m.userIndexKey(userID)

	now := time.Now()
	expAt := now.Add(m.conf.TTL).Unix()

	rc, err := m.luaBind.Run(ctx, redis2.GetRedis(),
		[]string{zUnauth, zUser},
		kUnauth, kAuth,
		int64(m.conf.TTL/time.Second), now.Unix(), expAt, boolToInt(m.conf.UseEXAT),
	).Int64()
	if err != nil {
		return false, err
	}
	switch rc {
	case 1:
		return true, nil
	case 0:
		return false, nil
	case -1:
		return false, errors.New("authorized key already exists")
	default:
		return false, fmt.Errorf("unexpected bind rc=%d", rc)
	}
}

// ===== 已授权阶段：下线/清理 =====

// Offline：单个会话下线（幂等）。当 userID=="" 时，按“未授权离线”处理。
func (m *OnlineStore) Offline(ctx context.Context, userID, snowID string, publish bool, reason string) (bool, error) {
	if strings.TrimSpace(userID) == "" {
		// 未授权会话的离线
		return m.OfflineUnauth(ctx, snowID, publish, reason)
	}

	kAuth := m.sessionKey(userID, snowID)
	zUser := m.userIndexKey(userID)

	rc, err := m.luaOfflineOne.Run(ctx, redis2.GetRedis(),
		[]string{zUser},
		kAuth,
	).Int64()
	if err != nil {
		return false, err
	}
	ok := rc == 1
	if publish && ok {
		// 事件：OFFLINE:<authKey>:<reason>
		msg := fmt.Sprintf("OFFLINE:%s:%s", kAuth, reason)
		_ = redis2.GetRedis().Publish(ctx, m.conf.ChannelName, msg).Err()
	}
	return ok, nil
}

// ForceLogoutUser：强制某用户“全部会话”下线（管理员/风控场景）。可选广播。
func (m *OnlineStore) ForceLogoutUser(ctx context.Context, userID string, publish bool, reason string) ([]string, error) {
	zUser := m.userIndexKey(userID)
	keys, err := m.luaLogoutAll.Run(ctx, redis2.GetRedis(), []string{zUser}).StringSlice()
	if err != nil {
		return nil, err
	}
	if publish && len(keys) > 0 {
		// 事件：FORCE_LOGOUT:<userID>:<reason>:<k1,k2,...>
		payload := fmt.Sprintf("FORCE_LOGOUT:%s:%s:%s", userID, reason, strings.Join(keys, ","))
		_ = redis2.GetRedis().Publish(ctx, m.conf.ChannelName, payload).Err()
	}
	return keys, nil
}

// SweepAuthedUser：清理“已授权但过期”的会话（按 score<=now）。可选广播。
func (m *OnlineStore) SweepAuthedUser(ctx context.Context, userID string, publish bool) ([]string, error) {
	zUser := m.userIndexKey(userID)
	now := time.Now().Unix()

	victims, err := m.luaSweepAuthed.Run(ctx, redis2.GetRedis(), []string{zUser}, now).StringSlice()
	if err != nil {
		return nil, err
	}
	if publish && len(victims) > 0 {
		// 事件：EXPIRE_CLEAN:<userID>:<k1,k2,...>
		payload := fmt.Sprintf("EXPIRE_CLEAN:%s:%s", userID, strings.Join(victims, ","))
		_ = redis2.GetRedis().Publish(ctx, m.conf.ChannelName, payload).Err()
	}
	return victims, nil
}

// ===== 工具 =====
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
