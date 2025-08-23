package mgo

import (
	mgo "PProject/data/database/mgo/mongoutil"
	"PProject/data/database/utils/tx"
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

type MongoManager struct {
	mu        sync.RWMutex
	client    *mgo.Client
	readyCh   chan struct{} // 首次就绪通知；只会被 close 一次
	readyOnce sync.Once

	lastErr atomic.Value // error
}

var globalMgr MongoManager

// StartAsync: 一直运行到 ctx.Done()；首次连上时 close readyCh，后续掉线会自动重连
func StartAsync(ctx context.Context, cfg *mgo.Config) {
	// init state
	if globalMgr.readyCh == nil {
		globalMgr.readyCh = make(chan struct{})
	}

	go func() {
		const (
			baseBackoff = 200 * time.Millisecond
			maxBackoff  = 5 * time.Second
			healthEvery = 10 * time.Second // 健康检查周期
			failThresh  = 3                // 连续失败阈值
		)

		for {
			// ===== 连接阶段（带退避重试） =====
			attempt := 0
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				cli, err := mgo.NewMongoDB(ctx, cfg)
				if err == nil {
					globalMgr.mu.Lock()
					globalMgr.client = cli
					globalMgr.mu.Unlock()

					// 只在“首次”成功时通知就绪
					globalMgr.readyOnce.Do(func() { close(globalMgr.readyCh) })

					break // 进入健康检查阶段
				}

				globalMgr.lastErr.Store(err)

				// 退避 + 抖动
				backoff := baseBackoff << attempt
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				jitter := time.Duration(rand.Int63n(int64(backoff / 5))) // 0~20%
				sleep := backoff - jitter/2

				timer := time.NewTimer(sleep)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
				if attempt < 6 {
					attempt++
				}
			}

			// ===== 健康检查阶段（保持/掉线→重连）=====
			fail := 0
			healthTicker := time.NewTicker(healthEvery)
			func() {
				defer healthTicker.Stop()
				for {
					select {
					case <-ctx.Done():
						// 退出前尽量断开
						globalMgr.mu.Lock()
						if globalMgr.client != nil {
							_ = globalMgr.client.GetDB().Client().Disconnect(context.Background())
							globalMgr.client = nil
						}
						globalMgr.mu.Unlock()
						return
					case <-healthTicker.C:
						// 可选：依赖驱动自动重连的话可注释此 ping
						globalMgr.mu.RLock()
						c := globalMgr.client
						globalMgr.mu.RUnlock()

						if c == nil {
							// 异常状态，跳出健康检查回到连接阶段
							return
						}
						if err := c.GetDB().Client().Ping(ctx, nil); err != nil {
							fail++
							globalMgr.lastErr.Store(err)
							if fail >= failThresh {
								// 标记掉线，断开并回到连接阶段
								globalMgr.mu.Lock()
								if globalMgr.client != nil {
									_ = globalMgr.client.GetDB().Client().Disconnect(context.Background())
									globalMgr.client = nil
								}
								globalMgr.mu.Unlock()
								return
							}
						} else {
							fail = 0
						}
					}
				}
			}() // 健康循环结束后自动回到外层 for 进行重连
		}
	}()
}

// Ready: 首次连接成功时会 close；可 select 等待
func Ready() <-chan struct{} {
	return globalMgr.readyCh
}

func Manager() *MongoManager {
	return &globalMgr
}

// Err: 最近一次错误
func Err() error {
	if v := globalMgr.lastErr.Load(); v != nil {
		return v.(error)
	}
	return nil
}

func GetDB() *mongo.Database {
	globalMgr.mu.RLock()
	defer globalMgr.mu.RUnlock()
	if globalMgr.client == nil {
		panic("Mongo not ready: wait Ready() or use TryGetDB()")
	}
	return globalMgr.client.GetDB()
}

func GetTx() tx.Tx {
	globalMgr.mu.RLock()
	defer globalMgr.mu.RUnlock()
	if globalMgr.client == nil {
		panic("Mongo not ready: wait Ready() or use TryGetTx()")
	}
	return globalMgr.client.GetTx()
}

func TryGetDB() (*mongo.Database, bool) {
	globalMgr.mu.RLock()
	defer globalMgr.mu.RUnlock()
	if globalMgr.client == nil {
		return nil, false
	}
	return globalMgr.client.GetDB(), true
}

func TryGetTx() (tx.Tx, bool) {
	globalMgr.mu.RLock()
	defer globalMgr.mu.RUnlock()
	if globalMgr.client == nil {
		return nil, false
	}
	return globalMgr.client.GetTx(), true
}

// 在 MongoManager 上新增：
func WaitReady(ctx context.Context, m *MongoManager) error {
	// 已就绪则立刻返回
	m.mu.RLock()
	readyCh := m.readyCh
	clientNil := m.client == nil
	m.mu.RUnlock()

	if !clientNil {
		return nil
	}
	if readyCh == nil {
		return fmt.Errorf("mongo manager not started")
	}

	select {
	case <-readyCh: // 首次成功时会被 close
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
