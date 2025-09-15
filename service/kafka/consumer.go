package kafka

import (
	"PProject/logger"
	"context"
	"time"

	"github.com/Shopify/sarama"
	"github.com/panjf2000/ants/v2"
)

type ConsumerGroupHandler struct{}

func (h *ConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	logger.Info("Consumer group setup")
	return nil
}

func (h *ConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	logger.Infof("Consumer group cleanup")
	return nil
}

// --- worker -> 主循环的回执 ---
type ack struct {
	off int64
	ok  bool
}

func (h *ConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// 1) 回执通道：worker 完成后回传
	ackC := make(chan ack, 2048)

	// 2) ants 池（每个被分配的分区各有一个 ConsumeClaim；这里给单分区开 32 并发）
	pool, err := ants.NewPool(32,
		ants.WithPreAlloc(true),
		ants.WithNonblocking(false), // pool 满时阻塞提交，形成自然背压
		ants.WithPanicHandler(func(p any) { logger.Errorf("worker panic: %v", p) }),
	)
	if err != nil {
		return err
	}
	defer pool.Release()

	// 3) （可选）限制分区内在途任务，避免内存膨胀
	inflight := make(chan struct{}, 4096)

	// 4) 连续推进提交点
	next := claim.InitialOffset()             // 期望推进到的“下一条要读”的 offset
	pendingOK := make(map[int64]struct{}, 64) // 已完成但前面没齐的 offset
	markedSinceCommit := 0

	// 5)（若关闭 AutoCommit）定时手动 Commit
	commitTicker := time.NewTicker(2 * time.Second)
	defer commitTicker.Stop()

	logger.Infof("[claim start] topic=%s partition=%d initial=%d",
		claim.Topic(), claim.Partition(), claim.InitialOffset)

	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				// 分区被回收
				logger.Errorf("[claim done] topic=%s partition=%d", claim.Topic(), claim.Partition())
				return nil
			}
			// 提前拷贝（如你的业务会修改 msg.Value/Key；若只读可不拷贝）
			// key := append([]byte(nil), msg.Key...)
			// val := append([]byte(nil), msg.Value...)

			// 背压
			inflight <- struct{}{}

			// 提交到 ants：只做业务处理，不触碰 session
			hdr, err := GetHandler(msg.Topic)
			if err != nil {
				logger.Errorf("No handler for topic=%s: %v", msg.Topic, err)
				<-inflight
				ackC <- ack{off: msg.Offset, ok: false}
				break
			}
			submitErr := pool.Submit(func(m *sarama.ConsumerMessage, h func(string, []byte, []byte) error) func() {
				return func() {
					defer func() { <-inflight }()
					if e := h(m.Topic, m.Key, m.Value); e != nil {
						logger.Errorf("handler error topic=%s partition=%d offset=%d err=%v",
							m.Topic, m.Partition, m.Offset, e)
						ackC <- ack{off: m.Offset, ok: false}
						return
					}
					ackC <- ack{off: m.Offset, ok: true}
				}
			}(msg, hdr))
			if submitErr != nil {
				// 极端情况下 pool.Submit 失败（例如被关闭）
				logger.Errorf("pool submit failed: %v", submitErr)
				<-inflight
				ackC <- ack{off: msg.Offset, ok: false}
			}

		case a := <-ackC:
			if !a.ok {
				// 失败：不推进提交点（可在这里做重试/DLQ）
				break
			}
			pendingOK[a.off] = struct{}{}
			// 从 next 开始，尽量推进“连续成功”的窗口
			for {
				if _, ok := pendingOK[next]; !ok {
					break
				}
				delete(pendingOK, next)
				// 提交的是“下一条要读”的 offset
				session.MarkOffset(claim.Topic(), claim.Partition(), next+1, "")
				markedSinceCommit++
				next++
			}

			// // 若关闭 AutoCommit，可用阈值触发手动提交
			// if markedSinceCommit >= 200 {
			// 	if err := session.Commit(); err != nil {
			// 		log.Printf("manual commit error: %v", err)
			// 	}
			// 	markedSinceCommit = 0
			// }

		case <-commitTicker.C:
			// 若关闭 AutoCommit，这里定时手动提交
			// if markedSinceCommit > 0 {
			// 	if err := session.Commit(); err != nil {
			// 		log.Printf("manual commit error: %v", err)
			// 	}
			// 	markedSinceCommit = 0
			// }

		case <-session.Context().Done():
			// rebalance，尽快退出
			logger.Infof("[claim ctx done] topic=%s partition=%d", claim.Topic(), claim.Partition())
			return nil
		}
	}
}

func StartConsumerGroup(brokers []string, groupID string, topics []string) error {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_1_0_0

	// 第一次无位点，从最新开始
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest

	// 推荐：开启自动提交 + 在代码里 MarkOffset（简单稳定）
	cfg.Consumer.Offsets.AutoCommit.Enable = true
	cfg.Consumer.Offsets.AutoCommit.Interval = 2 * time.Second

	// 心跳/会话（可按需要调整）
	cfg.Consumer.Group.Session.Timeout = 30 * time.Second
	cfg.Consumer.Group.Heartbeat.Interval = 3 * time.Second
	cfg.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange

	cfg.Consumer.Return.Errors = true

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return err
	}
	defer group.Close()

	// 错误日志
	go func() {
		for err := range group.Errors() {
			logger.Errorf("Consumer group error: %v", err)
		}
	}()

	// 优雅退出
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		//signal.Ignored()
		for {
			select {
			case <-ctx.Done():
				logger.Infof("Consumer group stopped")
				return
			default:
				// 模拟业务逻辑
				logger.Infof("Consumer group working")
				time.Sleep(time.Second)
			}
		}
	}()

	handler := &ConsumerGroupHandler{}
	for {
		if err := group.Consume(ctx, topics, handler); err != nil {
			logger.Errorf("Consume error: %v", err)
			// 短暂退避，避免 tight loop
			time.Sleep(time.Second)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}
