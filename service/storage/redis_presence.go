package storage

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"time"
)

type Config struct {
	Addr     string
	Password string
	DB       int
}

var (
	rdb *redis.Client
	ctx = context.Background()
)

func InitRedis(c Config) error {
	rdb = redis.NewClient(&redis.Options{Addr: c.Addr, Password: c.Password, DB: c.DB})
	return rdb.Ping(ctx).Err()
}

// presence key: im:presence:<user>
// Value: gateway_id, TTL controls the online validity period
func presenceKey(user string) string { return "im:presence:" + user }

// PresenceOnline sets the user as online and renews the TTL
func PresenceOnline(user, gatewayID string, ttl time.Duration) error {
	if rdb == nil {
		return fmt.Errorf("redis not initialized")
	}
	return rdb.Set(ctx, presenceKey(user), gatewayID, ttl).Err()
}

// PresenceOffline actively sets the user offline (deletes the key)
func PresenceOffline(user string) error {
	if rdb == nil {
		return fmt.Errorf("redis not initialized")
	}
	return rdb.Del(ctx, presenceKey(user)).Err()
}

// PresenceLookup checks whether the user is online
func PresenceLookup(user string) (gatewayID string, online bool, err error) {
	if rdb == nil {
		return "", false, fmt.Errorf("redis not initialized")
	}
	val, err := rdb.Get(ctx, presenceKey(user)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}
