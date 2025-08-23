package ids

import (
	"strconv"
	"sync"
	"time"
)

type generator struct {
	mu       sync.Mutex
	epochMS  int64
	nodeID   int64 // 0~1023
	seq      int64 // 0~4095
	lastTSMS int64
}

var (
	defaultGen *generator
	once       sync.Once
)

// initDefault 初始化默认生成器
func initDefault() {
	once.Do(func() {
		defaultGen = &generator{
			epochMS: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			nodeID:  1,
		}
	})
}

// Generate 静态方法：生成一个新的雪花ID
func Generate() int64 {
	initDefault()
	return defaultGen.next()
}

func GenerateString() string {
	return strconv.FormatInt(Generate(), 10)
}

// SetNodeID 设置 nodeID（0~1023），可在 main() 初始化时调用
func SetNodeID(nodeID int64) {
	initDefault()
	if nodeID < 0 || nodeID > 1023 {
		nodeID = 1
	}
	defaultGen.nodeID = nodeID
}

// ---------------- 内部方法 ----------------
func (g *generator) next() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	for {
		now := time.Now().UnixMilli()
		if now < g.lastTSMS {
			// 时钟回拨，等待
			time.Sleep(time.Duration(g.lastTSMS-now) * time.Millisecond)
			continue
		}
		if now == g.lastTSMS {
			g.seq = (g.seq + 1) & 0xFFF // 12 bits
			if g.seq == 0 {
				// 序列溢出，等到下一毫秒
				for now <= g.lastTSMS {
					now = time.Now().UnixMilli()
				}
			}
		} else {
			g.seq = 0
		}
		g.lastTSMS = now

		ts := (now - g.epochMS) & ((1 << 41) - 1)
		id := (ts << 22) | (g.nodeID << 12) | g.seq
		return id
	}
}
