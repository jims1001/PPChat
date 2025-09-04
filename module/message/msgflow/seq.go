package msgflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type SeqAllocator struct {
	rdb        redis.UniversalClient
	db         DB
	seqPrefix  string
	lockPrefix string
	lockTTL    time.Duration
	spinWait   time.Duration
}

func NewSeqAllocator(rdb redis.UniversalClient, db DB) *SeqAllocator {
	return &SeqAllocator{
		rdb:        rdb,
		db:         db,
		seqPrefix:  "im:seq",
		lockPrefix: "im:seq:init",
		lockTTL:    10 * time.Second,
		spinWait:   50 * time.Millisecond,
	}
}

func (a *SeqAllocator) seqKey(tenant, convID string) string {
	return fmt.Sprintf("%s:%s:%s", a.seqPrefix, tenant, convID)
}
func (a *SeqAllocator) lockKey(tenant, convID string) string {
	return fmt.Sprintf("%s:%s:%s", a.lockPrefix, tenant, convID)
}

// NextSeq：若 redis 未初始化（无/0），自动创建会话→读 DB max(seq)→SET→INCR
func (a *SeqAllocator) NextSeq(ctx context.Context, tenant, convID string) (int64, error) {
	key := a.seqKey(tenant, convID)
	if v, err := a.rdb.Get(ctx, key).Int64(); err == nil && v > 0 {
		return a.rdb.Incr(ctx, key).Result()
	}
	if err := a.initIfNeeded(ctx, tenant, convID); err != nil {
		return 0, err
	}
	return a.rdb.Incr(ctx, key).Result()
}

func (a *SeqAllocator) initIfNeeded(ctx context.Context, tenant, convID string) error {
	key := a.seqKey(tenant, convID)
	if v, err := a.rdb.Get(ctx, key).Int64(); err == nil && v > 0 {
		return nil
	}
	// 加锁防止风暴
	lock := a.lockKey(tenant, convID)
	token := randToken(16)
	ok, err := a.rdb.SetNX(ctx, lock, token, a.lockTTL).Result()
	if err != nil {
		return err
	}
	if !ok {
		timer := time.NewTimer(a.spinWait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if v, err := a.rdb.Get(ctx, key).Int64(); err == nil && v > 0 {
			return nil
		}
		return errors.New("seq init contention, retry")
	}
	defer func() { _ = unlock(ctx, a.rdb, lock, token) }()

	// 双检
	if v, err := a.rdb.Get(ctx, key).Int64(); err == nil && v > 0 {
		return nil
	}
	// 会话存在化 + 初始化
	if err := a.db.EnsureConversation(ctx, tenant, convID); err != nil {
		return err
	}
	maxSeq, err := a.db.QueryMaxSeq(ctx, tenant, convID)
	if err != nil {
		return err
	}
	return a.rdb.Set(ctx, key, maxSeq, 0).Err()
}

// 发现落后时：只升不降，矫正后 INCR 取新号
var reconcileAndNextLua = `
local k = KEYS[1]
local dbMax = tonumber(ARGV[1])
local v = redis.call('GET', k)
if (not v) or (tonumber(v) < dbMax) then
  redis.call('SET', k, dbMax)
end
return redis.call('INCR', k)
`

func (a *SeqAllocator) ReconcileAndNext(ctx context.Context, tenant, convID string, dbMax int64) (int64, error) {
	return a.rdb.Eval(ctx, reconcileAndNextLua, []string{a.seqKey(tenant, convID)}, dbMax).Int64()
}
