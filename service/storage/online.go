package storage

import (
	redis2 "PProject/service/storage/redis"
	errors "PProject/tools/errs"
	"PProject/tools/ids"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
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
	UserIndexTTL  time.Duration
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

// ====== 新增三段：在线判断 / 拉取全部有效会话 / 拉取最新有效会话 ======

// 清理过期并返回“所有有效会话键”
// KEYS[1] = user index key
// ARGV[1] = nowUnix
// 返回：还有效的会话键数组（顺带把过期的删掉）
const luaGetActiveAndSweep = `
local userZ = KEYS[1]
local now   = tonumber(ARGV[1])

-- 清理过期（<= now）
local victims = redis.call("ZRANGEBYSCORE", userZ, "-inf", now)
for _, v in ipairs(victims) do
  redis.call("ZREM", userZ, v)
  redis.call("DEL", v)
end

-- 返回仍有效（> now）的会话成员
local actives = redis.call("ZRANGEBYSCORE", userZ, now + 1, "+inf")

if redis.call("ZCARD", userZ) > 0 then
  redis.call("EXPIRE", userZ, 3600)
end
return actives
`

// 清理过期并返回在线标志与数量
// KEYS[1] = user index key
// ARGV[1] = nowUnix
// 返回：数组 [在线标志(0/1), 数量]
const luaIsOnline = `
local userZ = KEYS[1]
local now   = tonumber(ARGV[1])

-- 清理过期
local victims = redis.call("ZRANGEBYSCORE", userZ, "-inf", now)
for _, v in ipairs(victims) do
  redis.call("ZREM", userZ, v)
  redis.call("DEL", v)
end

local cnt = redis.call("ZCOUNT", userZ, now + 1, "+inf")
if redis.call("ZCARD", userZ) > 0 then
  redis.call("EXPIRE", userZ, 3600)
end

if cnt > 0 then
  return {1, cnt}
else
  return {0, 0}
end
`

// 清理过期并返回最新（score 最大）的有效会话
// KEYS[1] = user index key
// ARGV[1] = nowUnix
// 返回：字符串（最新有效会话键，不存在则空串）
const luaGetNewestActive = `
local userZ = KEYS[1]
local now   = tonumber(ARGV[1])

-- 清理过期
local victims = redis.call("ZRANGEBYSCORE", userZ, "-inf", now)
for _, v in ipairs(victims) do
  redis.call("ZREM", userZ, v)
  redis.call("DEL", v)
end

local items = redis.call("ZREVRANGEBYSCORE", userZ, "+inf", now + 1, "LIMIT", 0, 1)

if redis.call("ZCARD", userZ) > 0 then
  redis.call("EXPIRE", userZ, 3600)
end

if items and items[1] then
  return items[1]
else
  return ""
end
`

const luaHeartbeatAuth = `-- KEYS[1] = zUser      -> nidx:{gateway_01:user_10001}
-- KEYS[2] = kConn      -> n:{gateway_01:user_10001}:id:<snowID>
-- ARGV[1] = ttlSec     -> kConn 会话 TTL（秒）
-- ARGV[2] = nowUnix    -> 当前时间（秒）
-- ARGV[3] = expAt      -> 本次续期到期时间（秒）
-- ARGV[4] = useEXAT    -> 1=EXPIREAT, 0=EXPIRE
-- ARGV[5] = memberStr  -> 索引里使用的 member（推荐直接传 kConn 字符串）
-- ARGV[6] = zUserTtl   -> 主索引兜底 TTL（秒，0=不设置）
-- ARGV[7] = refreshIdx -> 1=每次都 EXPIRE zUser；0=仅当 zUser 无 TTL 时设置
-- ARGV[8] = cleanExp   -> 1=清理 zUser 里已过期成员；0=不清理

local zUser      = KEYS[1]
local kConn      = KEYS[2]
local ttlSec     = tonumber(ARGV[1])
local nowUnix    = tonumber(ARGV[2])
local expAt      = tonumber(ARGV[3])
local useEXAT    = tonumber(ARGV[4]) == 1
local memberStr  = ARGV[5]
local zUserTtl   = tonumber(ARGV[6] or "0")
local refreshIdx = tonumber(ARGV[7] or "1") == 1
local cleanExp   = tonumber(ARGV[8] or "1") == 1

-- 1) 会话存在性检查
if redis.call('EXISTS', kConn) == 0 then
  return 0
end

-- 2) 续期会话 TTL
if useEXAT then
  redis.call('EXPIREAT', kConn, expAt)
else
  redis.call('EXPIRE',   kConn, ttlSec)
end

-- 3) 清理索引中过期成员（按 score）
if cleanExp then
  redis.call('ZREMRANGEBYSCORE', zUser, '-inf', nowUnix)
end

-- 4) 索引写/续：member=连接 key 字符串，score=expAt
redis.call('ZADD', zUser, expAt, memberStr)

-- 5) 刷新主索引 TTL
if zUserTtl > 0 then
  if refreshIdx then
    redis.call('EXPIRE', zUser, zUserTtl)
  else
    local ttl = redis.call('TTL', zUser)
    if ttl == -1 then
      redis.call('EXPIRE', zUser, zUserTtl)
    end
  end
end

return 1
 `

// OnlineStore ===== Store =====
type OnlineStore struct {
	conf OnlineConfig
	// 你原有的脚本（保留占位）
	luaBatch *redis.Script
	luaHB    *redis.Script
	// 既有脚本
	luaBind             *redis.Script
	luaSweep            *redis.Script
	luaOfflineUnauthOne *redis.Script
	luaOfflineOne       *redis.Script
	luaLogoutAll        *redis.Script
	luaSweepAuthed      *redis.Script
	// 新增脚本
	luaGetActiveAndSweep *redis.Script
	luaIsOnline          *redis.Script
	luaGetNewestActive   *redis.Script
	luaHeartbeatAuth     *redis.Script
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

	// 新增
	m.luaGetActiveAndSweep = redis.NewScript(luaGetActiveAndSweep)
	m.luaIsOnline = redis.NewScript(luaIsOnline)
	m.luaGetNewestActive = redis.NewScript(luaGetNewestActive)
	m.luaHeartbeatAuth = redis.NewScript(luaHeartbeatAuth)
}

// ===== Key 构造 =====

// 未授权会话键
// UseClusterTag=true: n:{<node>}:id:<snow>
// false:              n:%s:id:%s
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
// false:              n:%s:id:%s:u:%s
func (m *OnlineStore) sessionKey(userID, snowID string) string {
	if m.conf.UseClusterTag {
		return fmt.Sprintf("n:{%s:%s}:id:%s", m.conf.NodeID, userID, snowID)
	}
	return fmt.Sprintf("n:%s:id:%s:u:%s", m.conf.NodeID, snowID, userID)
}

// 用户索引ZSET（member=会话key, score=expireAtUnix）
// UseClusterTag=true: nidx:{<node>:<user>}
// false:              nidx:%s:u:%s
func (m *OnlineStore) userIndexKey(userID string) string {
	if m.conf.UseClusterTag {
		return fmt.Sprintf("nidx:{%s:%s}", m.conf.NodeID, userID)
	}
	return fmt.Sprintf("nidx:%s:u:%s", m.conf.NodeID, userID)
}

// ===== 未授权阶段 API =====

// Connect Connect：创建一个未授权连接（仅连接未登录）
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

// HeartbeatUnauth HeartbeatUnauth：未授权连接心跳（延长宽限）
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

// HeartbeatAuthorized ：已授权会话心跳续期
// 建议：由调用方传入 *短超时* ctx（如 2s）
// 返回：true=续期成功；false=会话不存在；err!=nil 时是 Redis 或脚本执行错误。
// 建议：让上层传短超时 ctx，如 2s。这里保持你现有签名即可。
func (m *OnlineStore) HeartbeatAuthorized(gatewayID, userID, snowID string) (bool, *errors.CodeError) {
	if ctx == nil {
		ctx = context.Background()
	}
	// 统一哈希标签，确保 Cluster 同槽
	zUser := fmt.Sprintf("nidx:{%s:%s}", gatewayID, userID)
	kConn := fmt.Sprintf("n:{%s:%s}:id:%s", gatewayID, userID, snowID)

	now := time.Now()
	expAt := now.Add(m.conf.TTL).Unix()
	ttlSec := int64(m.conf.TTL / time.Second)
	zUserTTL := int64(0)
	if m.conf.UserIndexTTL > 0 {
		zUserTTL = int64(m.conf.UserIndexTTL / time.Second) // e.g. 86400
	}

	rc, err := m.luaHeartbeatAuth.Run(
		ctx, redis2.GetRedis(),
		[]string{zUser, kConn},
		ttlSec,                    // ARGV[1]
		now.Unix(),                // ARGV[2]
		expAt,                     // ARGV[3]
		boolToInt(m.conf.UseEXAT), // ARGV[4]
		kConn,                     // ARGV[5] -> memberStr = 连接 key 字符串
		zUserTTL,                  // ARGV[6]
		1,                         // ARGV[7] refreshIdx=1: 每次都 EXPIRE zUser（改成 0=仅无TTL时设置）
		1,                         // ARGV[8] cleanExp=1: 清理索引中过期成员
	).Int64()
	if err != nil {
		return false, &errors.CodeError{Msg: err.Error(), Code: 201}
	}
	switch rc {
	case 1:
		return true, nil
	case 0:
		return false, nil // 会话 key 不存在（被踢/过期/未建立）
	default:
		return false, &errors.CodeError{Msg: fmt.Sprintf("unexpected hb rc=%d", rc), Code: 202}
	}
}

// SweepUnauth SweepUnauth：清理所有超时未授权连接，并选择性广播“踢出”
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

// OfflineUnauth OfflineUnauth：未授权单会话离线（幂等）。可选广播。
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

// Authorize Authorize：把“未授权连接”原子绑定成“已授权会话”
func (m *OnlineStore) Authorize(ctx context.Context, userID, snowID string) (bool, *errors.CodeError) {
	if m.conf.UnauthTTL <= 0 {
		return false, &errors.CodeError{Msg: "unauth stage disabled", Code: 100}
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
		return false, &errors.CodeError{Msg: err.Error(), Code: 101}
	}
	switch rc {
	case 1:
		return true, nil
	case 0:
		return false, nil
	case -1:
		return false, &errors.ErrorRecordIsExist
	default:
		return false, &errors.CodeError{Msg: fmt.Sprintf("unexpected bind rc=%d", rc), Code: 102}
	}
}

// ===== 已授权阶段：下线/清理 =====

// Offline Offline：单个会话下线（幂等）。当 userID=="" 时，按“未授权离线”处理。
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

// ForceLogoutUser ForceLogoutUser：强制某用户“全部会话”下线（管理员/风控场景）。可选广播。
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

// SweepAuthedUser ：清理“已授权但过期”的会话（按 score<=now）。可选广播。
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

// ===== 新增：查询在线状态 / 拉取会话 =====

// IsOnline IsOnline：判断用户是否在线，并返回在线会话数量（顺带清理过期）
func (m *OnlineStore) IsOnline(ctx context.Context, userID string) (online bool, count int64, err error) {
	zUser := m.userIndexKey(userID)
	now := time.Now().Unix()

	vals, e := m.luaIsOnline.Run(ctx, redis2.GetRedis(), []string{zUser}, now).Slice()
	if e != nil {
		return false, 0, e
	}

	var flag int64
	if len(vals) >= 2 {
		switch v := vals[0].(type) {
		case int64:
			flag = v
		case string:
			if v == "1" {
				flag = 1
			}
		}
		switch v := vals[1].(type) {
		case int64:
			count = v
		case string:
			// 兜底（通常不会返回字符串）
			if v == "0" {
				count = 0
			} else {
				count = 1
			}
		}
	}
	return flag == 1, count, nil
}

//func (m *OnlineStore) BatchListOnlineConnList(ctx context.Context, user string) ([]string, error) {
//	now := strconv.FormatInt(time.Now().Unix(), 10) // 或 UnixMilli()
//	return redis2.GetRedis().ZRangeByScore(ctx, "u2n:{"+user+"}", &redis.ZRangeBy{
//		Min: now, Max: "+inf",
//	}).Result()
//}

func (m *OnlineStore) BatchListOnlineConnList(ctx context.Context, userID string) ([]string, error) {
	// nidx:{*:user_10002}
	iter := redis2.GetRedis().Scan(ctx, 0, fmt.Sprintf("nidx:{*:%s}", userID), 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	return keys, iter.Err()
}

func (m *OnlineStore) GetUserGateway(ctx context.Context, userID string) (string, error) {
	keys, err := m.BatchListOnlineConnList(ctx, userID)
	if len(keys) == -0 || err != nil {
		return "", fmt.Errorf("user %s has no gateway", userID)
	}

	gateway := ExtractGateway(keys[0])
	if gateway == "" {
		return "", fmt.Errorf("user %s has no gateway", userID)
	}
	return gateway, nil
}

// ExtractGateway 从 key 里提取 gatewayId
// 例如: "nidx:{gateway_01:user_10001}" -> "gateway_01"
func ExtractGateway(key string) string {
	start := strings.IndexByte(key, '{')
	end := strings.IndexByte(key, '}')
	if start == -1 || end == -1 || end <= start+1 {
		return ""
	}
	// 花括号里的内容: "gateway_01:user_10001"
	body := key[start+1 : end]
	parts := strings.SplitN(body, ":", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// GetActiveSessions GetActiveSessions：获取用户所有“仍有效”的会话键（顺带清理过期）
func (m *OnlineStore) GetActiveSessions(ctx context.Context, userID string) ([]string, error) {
	zUser := m.userIndexKey(userID)
	now := time.Now().Unix()

	actives, err := m.luaGetActiveAndSweep.Run(ctx, redis2.GetRedis(), []string{zUser}, now).StringSlice()
	if err != nil {
		return nil, err
	}
	return actives, nil
}

// GetNewestSession GetNewestSession：获取用户最新（过期时间最大）的有效会话键；不存在则返回空串
func (m *OnlineStore) GetNewestSession(ctx context.Context, userID string) (string, error) {
	zUser := m.userIndexKey(userID)
	now := time.Now().Unix()

	newest, err := m.luaGetNewestActive.Run(ctx, redis2.GetRedis(), []string{zUser}, now).Text()
	if err != nil && err != redis.Nil {
		return "", err
	}
	if newest == "" || newest == "(nil)" {
		return "", nil
	}
	return newest, nil
}

// ===== 工具 =====
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
