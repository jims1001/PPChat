package message

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ClientMsgIndex 负责管理 “ClientMsgID -> ServerMsgID” 的幂等窗口
type ClientMsgIndex struct {
	rdb    redis.UniversalClient
	prefix string
	ttl    time.Duration
	genSID func() string // 用于首处理时生成 ServerMsgID
}

// Option 配置项
type Option func(*ClientMsgIndex)

// WithPrefix 自定义键名前缀（默认 "im:cid"）
func WithPrefix(prefix string) Option {
	return func(m *ClientMsgIndex) { m.prefix = prefix }
}

// WithTTL 设置去重窗口TTL（默认 48h）
func WithTTL(ttl time.Duration) Option {
	return func(m *ClientMsgIndex) { m.ttl = ttl }
}

// WithSIDGenerator 自定义 ServerMsgID 生成器
func WithSIDGenerator(gen func() string) Option {
	return func(m *ClientMsgIndex) { m.genSID = gen }
}

// NewClientMsgIndex 构造
func NewClientMsgIndex(rdb redis.UniversalClient, opts ...Option) *ClientMsgIndex {
	m := &ClientMsgIndex{
		rdb:    rdb,
		prefix: "im:cid",
		ttl:    48 * time.Hour,
		genSID: func() string { return uuid.NewString() }, // 默认 UUID，可替换雪花/ULID
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// key 规范：im:cid:{tenant}:{sender}:{clientMsgID}
func (m *ClientMsgIndex) key(tenant, sender, clientMsgID string) string {
	return fmt.Sprintf("%s:%s:%s:%s", m.prefix, tenant, sender, clientMsgID)
}

// Ensure 确认/创建幂等映射：
// - 若已存在：返回 (serverMsgID, existed=true, nil)
// - 若不存在：生成 serverMsgID，写入并返回 (serverMsgID, existed=false, nil)
//
// 可传入 proposedSID（可选）；为空时使用 m.genSID()
func (m *ClientMsgIndex) Ensure(ctx context.Context, tenant, sender, clientMsgID string, proposedSID string) (serverMsgID string, existed bool, err error) {
	key := m.key(tenant, sender, clientMsgID)
	sid := proposedSID
	if sid == "" {
		sid = m.genSID()
	}
	// Lua：原子 SETNX + PEXPIRE；若已存在则 GET 返回旧值
	const lua = `
local k = KEYS[1]
local v = ARGV[1]
local ttl_ms = tonumber(ARGV[2])
local ok = redis.call('SETNX', k, v)
if ok == 1 then
  redis.call('PEXPIRE', k, ttl_ms)
  return {0, v}  -- 0=新写入
else
  local old = redis.call('GET', k)
  return {1, old} -- 1=命中已有
end
`
	res, err := m.rdb.Eval(ctx, lua, []string{key}, sid, int64(m.ttl/time.Millisecond)).Result()
	if err != nil {
		return "", false, err
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 2 {
		return "", false, fmt.Errorf("unexpected lua result: %#v", res)
	}
	flag, _ := arr[0].(int64)
	val, _ := arr[1].(string)
	return val, flag == 1, nil
}

// Get 读取已存在的映射（不存在返回 redis.Nil）
func (m *ClientMsgIndex) Get(ctx context.Context, tenant, sender, clientMsgID string) (string, error) {
	return m.rdb.Get(ctx, m.key(tenant, sender, clientMsgID)).Result()
}

// Exists 判断是否存在
func (m *ClientMsgIndex) Exists(ctx context.Context, tenant, sender, clientMsgID string) (bool, error) {
	n, err := m.rdb.Exists(ctx, m.key(tenant, sender, clientMsgID)).Result()
	return n == 1, err
}

// Del 删除（例如回滚/清理）
func (m *ClientMsgIndex) Del(ctx context.Context, tenant, sender, clientMsgID string) error {
	_, err := m.rdb.Del(ctx, m.key(tenant, sender, clientMsgID)).Result()
	return err
}

// Touch 续期（可选：在长时链路中延长幂等窗口）
func (m *ClientMsgIndex) Touch(ctx context.Context, tenant, sender, clientMsgID string) error {
	_, err := m.rdb.PExpire(ctx, m.key(tenant, sender, clientMsgID), m.ttl).Result()
	return err
}
