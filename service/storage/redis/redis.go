package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
	"sync"
	"time"
)

var (
	redisOnce sync.Once
	redisMgr  *RedisManager
)

type RedisManager struct {
	client *redis.Client
}

// Config 用于初始化 Redis
type Config struct {
	Addr     string
	Password string
	DB       int
	PoolSize int
}

// InitRedis 初始化 Redis 管理器（单例）
func InitRedis(c Config) error {
	var initErr error
	redisOnce.Do(func() {
		rdb := redis.NewClient(&redis.Options{
			Addr:     c.Addr,
			Password: c.Password,
			DB:       c.DB,
			PoolSize: c.PoolSize,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := rdb.Ping(ctx).Err(); err != nil {
			initErr = err
			return
		}

		redisMgr = &RedisManager{client: rdb}
	})
	return initErr
}

// GetRedis 获取 Redis Client
func GetRedis() *redis.Client {
	if redisMgr == nil {
		panic("Redis not initialized, call InitRedis first")
	}
	return redisMgr.client
}

// CloseRedis 关闭连接
func CloseRedis() error {
	if redisMgr != nil && redisMgr.client != nil {
		return redisMgr.client.Close()
	}
	return nil
}
