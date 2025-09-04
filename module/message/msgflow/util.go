package msgflow

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/redis/go-redis/v9"
)

func HashPayload(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func randToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// 分布式锁解锁（校验 token）
func unlock(ctx context.Context, rdb redis.UniversalClient, lockKey, token string) error {
	const lua = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
else
  return 0
end`
	return rdb.Eval(ctx, lua, []string{lockKey}, token).Err()
}
