package mongoutil

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"strings"
)

const (
	defaultMaxPoolSize = 100
	defaultMaxRetry    = 3
)

func buildMongoURI(config *Config, authSource string) string {
	credentials := ""

	if config.Username != "" && config.Password != "" {
		credentials = fmt.Sprintf("%s:%s", config.Username, config.Password)
	}

	return fmt.Sprintf(
		"mongodb://%s@%s/%s?authSource=%s&maxPoolSize=%d",
		credentials,
		strings.Join(config.Address, ","),
		config.Database,
		authSource,
		config.MaxPoolSize,
	)
}

// shouldRetry determines whether an error should trigger a retry.
func shouldRetry(ctx context.Context, err error) bool {
	select {
	case <-ctx.Done():
		return false
	default:
		if cmdErr, ok := err.(mongo.CommandError); ok {
			return cmdErr.Code != 13 && cmdErr.Code != 18
		}
		return true
	}
}
