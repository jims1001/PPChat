package storage

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/redis/go-redis/v9"
)

// —— Conversation history: Redis Streams ——

func DMKey(a, b string) string {
	p := []string{a, b}
	sort.Strings(p)
	return fmt.Sprintf("im:dm:%s:%s", p[0], p[1])
}

func AppendStream(stream string, fields map[string]any) (string, error) {
	if rdb == nil {
		return "", fmt.Errorf("redis not initialized")
	}
	args := &redis.XAddArgs{Stream: stream, Values: fields, Approx: true, MaxLen: 100_000}
	return rdb.XAdd(ctx, args).Result()
}

// —— Offline queue: one List per user ——

type OfflineMsg struct {
	From    string `json:"from"`
	Payload []byte `json:"payload"`
}

func offlineKey(user string) string { return "im:offline:" + user }

// EnqueueOffline stores a msg into the user's offline queue
func EnqueueOffline(user, from string, payload []byte) error {
	if rdb == nil {
		return fmt.Errorf("redis not initialized")
	}
	b, _ := json.Marshal(OfflineMsg{From: from, Payload: payload})
	// Use LPUSH + LTRIM to implement a rolling window (keep the most recent N messages)
	pipe := rdb.TxPipeline()
	pipe.LPush(ctx, offlineKey(user), b)
	pipe.LTrim(ctx, offlineKey(user), 0, 9999) // Keep only the most recent 10,000 messages
	_, err := pipe.Exec(ctx)
	return err
}

// FetchOffline retrieves and clears up to n offline messages in bulk
// (starting from the tail to ensure FIFO order)
func FetchOffline(user string, n int) ([]OfflineMsg, error) {
	if rdb == nil {
		return nil, fmt.Errorf("redis not initialized")
	}
	if n <= 0 {
		n = 100
	}

	// Get current length, then get the range
	llen, err := rdb.LLen(ctx, offlineKey(user)).Result()
	if err != nil {
		return nil, err
	}
	if llen == 0 {
		return nil, nil
	}
	if int64(n) > llen {
		n = int(llen)
	}

	// Get the last n items in reverse order (to maintain FIFO): index range [llen-n, llen-1]
	start := llen - int64(n)
	vals, err := rdb.LRange(ctx, offlineKey(user), start, llen-1).Result()
	if err != nil {
		return nil, err
	}

	// Trim the retrieved part
	if err := rdb.LTrim(ctx, offlineKey(user), 0, start-1).Err(); err != nil {
		return nil, err
	}

	out := make([]OfflineMsg, 0, len(vals))
	for _, v := range vals {
		var m OfflineMsg
		_ = json.Unmarshal([]byte(v), &m)
		out = append(out, m)
	}
	return out, nil
}
