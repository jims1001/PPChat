package seq

import (
	errors "PProject/tools/errs"
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// 段内原子发号：KEYS[1]=key; ARGV[1]=need; ARGV[2]=segEnd; ARGV[3]=nowMs
// 返回：{0,start,0,end,nowMs} 成功；{1} notfound；{3,curr,end,0,nowMs} 用尽/不一致
var luaInSegment = redis.NewScript(`
  local k = KEYS[1]
  local need = tonumber(ARGV[1])
  local segEnd = tonumber(ARGV[2])
  local nowms = tonumber(ARGV[3])

  local curr = redis.call('HGET', k, 'curr')
  local endv = redis.call('HGET', k, 'end')
  if not curr or not endv then
    return {1}
  end
  curr = tonumber(curr); endv = tonumber(endv)

  if segEnd ~= 0 and segEnd ~= endv then
    return {3, curr, endv, 0, nowms}
  end

  local start = curr + 1
  local newv  = curr + need
  if newv > endv then
    return {3, curr, endv, 0, nowms}
  end
  redis.call('HSET', k, 'curr', newv, 'mill', nowms)
  return {0, start, 0, endv, nowms}
`)

// 装载/刷新段：curr=start-1, end=end, mill=nowMs，并设置TTL
var luaSetSegment = redis.NewScript(`
  local k = KEYS[1]
  local curr = tonumber(ARGV[1])
  local endv = tonumber(ARGV[2])
  local nowms= tonumber(ARGV[3])
  redis.call('HSET', k, 'curr', curr, 'end', endv, 'mill', nowms)
  redis.call('PEXPIRE', k, 3600000) -- 1h，可按需调整或动态续期
  return 1
`)

type DAOIface interface {
	AllocSegment(ctx context.Context, tenantID, conversationID string, block int64) (start, end int64, err error)
}

type Allocator struct {
	Rdb         redis.Scripter
	DAO         DAOIface
	BlockSizeFn func(tenantID, conversationID string, want int64) int64
	KeyFn       func(tenantID, conversationID string) string
	MaxRetry    int
}

func defaultBlock(_ string, _ string, want int64) int64 {
	if want <= 0 {
		want = 1
	}
	if want < 32 {
		return 256
	} // 冷会话小段
	return want * 8 // 热会话放大
}
func defaultKey(tenant, conv string) string { return "seq:blk:" + tenant + ":" + conv }

func (a *Allocator) ensure() {
	if a.BlockSizeFn == nil {
		a.BlockSizeFn = defaultBlock
	}
	if a.KeyFn == nil {
		a.KeyFn = defaultKey
	}
	if a.MaxRetry == 0 {
		a.MaxRetry = 10
	}
}

// Malloc Malloc：分配 need 个连续 seq（返回起始 start，与 mill 时间戳）
func (a *Allocator) Malloc(ctx context.Context, tenantID, conversationID string, need int64) (start int64, mill int64, err error) {
	a.ensure()
	if need <= 0 {
		need = 1
	}
	key := a.KeyFn(tenantID, conversationID)
	nowms := time.Now().UnixMilli()

	// 1) 先尝试在现有段内发号
	if res, e := luaInSegment.Run(ctx, a.Rdb, []string{key}, need, 0, nowms).Result(); e == nil {
		arr := res.([]interface{})
		switch arr[0].(int64) {
		case 0:
			return arr[1].(int64), arr[4].(int64), nil
		case 1, 3:
			// not found / exceeded -> 回源
		default:
			return 0, 0, errors.New("unknown redis state %v", arr[0])
		}
	}

	// 2) 回源 Mongo 领段 -> 写回 Redis -> 再尝试段内发号
	var lastErr error
	for i := 0; i < a.MaxRetry; i++ {
		block := a.BlockSizeFn(tenantID, conversationID, need)

		segStart, segEnd, e := a.DAO.AllocSegment(ctx, tenantID, conversationID, block)
		if e != nil {
			lastErr = e
			break
		}

		if _, e = luaSetSegment.Run(ctx, a.Rdb, []string{key}, segStart-1, segEnd, nowms).Result(); e != nil {
			lastErr = e
			time.Sleep(10 * time.Millisecond)
			continue
		}

		res2, e := luaInSegment.Run(ctx, a.Rdb, []string{key}, need, segEnd, nowms).Result()
		if e != nil {
			lastErr = e
			time.Sleep(10 * time.Millisecond)
			continue
		}
		arr := res2.([]interface{})
		if arr[0].(int64) == 0 {
			return arr[1].(int64), arr[4].(int64), nil
		}
		time.Sleep(5 * time.Millisecond) // 段冲突，小憩后重试
	}
	if lastErr == nil {
		lastErr = errors.New("malloc retry exceeded")
	}
	return 0, 0, lastErr
}
