package msgflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// cid → sid 的幂等窗口：存 {sid, st, ph, t}
type idxValue struct {
	ServerMsgID string `json:"sid"`
	Status      string `json:"st"` // PENDING | COMMITTED
	PayloadHash string `json:"ph"`
	CreatedAtMS int64  `json:"t"`
}

type ClientMsgIndex struct {
	rdb           redis.UniversalClient
	prefix        string
	ttl           time.Duration
	genSID        func() string
	shortRollback time.Duration
}

type IdxOption func(*ClientMsgIndex)

func NewClientMsgIndex(rdb redis.UniversalClient, opts ...IdxOption) *ClientMsgIndex {
	m := &ClientMsgIndex{
		rdb:           rdb,
		prefix:        "im:cid",
		ttl:           48 * time.Hour,
		genSID:        func() string { return uuid.NewString() },
		shortRollback: 90 * time.Second,
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

func (m *ClientMsgIndex) key(tenant, sender, cid string) string {
	return fmt.Sprintf("%s:%s:%s:%s", m.prefix, tenant, sender, cid)
}

// EnsureEx：原子占位（PENDING），若已存在返回 old
func (m *ClientMsgIndex) EnsureEx(ctx context.Context, tenant, sender, cid, payloadHash, proposedSID string) (iv idxValue, existed bool, err error) {
	key := m.key(tenant, sender, cid)
	sid := proposedSID
	if sid == "" {
		sid = m.genSID()
	}
	now := time.Now().UnixMilli()
	newJSON, _ := json.Marshal(idxValue{ServerMsgID: sid, Status: "PENDING", PayloadHash: payloadHash, CreatedAtMS: now})

	const lua = `
local k   = KEYS[1]
local vv  = redis.call('GET', k)
if vv then return {1, vv} end
redis.call('SET', k, ARGV[1], 'PX', ARGV[2])
return {0, ARGV[1]}
`
	res, err := m.rdb.Eval(ctx, lua, []string{key}, string(newJSON), int64(m.ttl/time.Millisecond)).Result()
	if err != nil {
		return iv, false, err
	}
	arr := res.([]interface{})
	flag := arr[0].(int64)
	raw := arr[1].(string)
	_ = json.Unmarshal([]byte(raw), &iv)
	return iv, flag == 1, nil
}

// MarkCommitted：PENDING→COMMITTED（CAS：sid/ph）
func (m *ClientMsgIndex) MarkCommitted(ctx context.Context, tenant, sender, cid, sid, payloadHash string) error {
	key := m.key(tenant, sender, cid)
	const lua = `
local vv = redis.call('GET', KEYS[1])
if not vv then return 0 end
local obj = cjson.decode(vv)
if obj["sid"] ~= ARGV[1] or obj["ph"] ~= ARGV[2] then return -1 end
obj["st"] = "COMMITTED"
redis.call('SET', KEYS[1], cjson.encode(obj), 'PX', ARGV[3])
return 1
`
	ttl := int64(m.ttl / time.Millisecond)
	_, err := m.rdb.Eval(ctx, lua, []string{key}, sid, payloadHash, ttl).Result()
	return err
}

// UpdateSIDIfPending：仅在 PENDING 且 ph 一致时替换 sid（处理极小概率 sid 冲突）
func (m *ClientMsgIndex) UpdateSIDIfPending(ctx context.Context, tenant, sender, cid, payloadHash, newSid string) error {
	key := m.key(tenant, sender, cid)
	const lua = `
local vv = redis.call('GET', KEYS[1])
if not vv then return 0 end
local obj = cjson.decode(vv)
if obj["st"] ~= "PENDING" or obj["ph"] ~= ARGV[2] then return -1 end
obj["sid"] = ARGV[1]
redis.call('SET', KEYS[1], cjson.encode(obj), 'PX', ARGV[3])
return 1
`
	ttl := int64(m.ttl / time.Millisecond)
	_, err := m.rdb.Eval(ctx, lua, []string{key}, newSid, payloadHash, ttl).Result()
	return err
}

// RollbackShortTTL：缩短 TTL，让客户端尽快重试
func (m *ClientMsgIndex) RollbackShortTTL(ctx context.Context, tenant, sender, cid string) error {
	return m.rdb.PExpire(ctx, m.key(tenant, sender, cid), m.shortRollback).Err()
}
