package kafka

import (
	"PProject/logger"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Shopify/sarama"
)

//func EnsureTopics(admin sarama.ClusterAdmin, topics []string) error {
//	for _, t := range topics {
//		desc, err := admin.DescribeTopics([]string{t})
//		if err == nil && len(desc) == 1 && desc[0].Err == sarama.ErrNoError {
//			log.Printf("[Topic] exists: %s (partitions=%d)", t, len(desc[0].Partitions))
//			continue
//		}
//		td := &sarama.TopicDetail{
//			NumPartitions:     Cfg.PartitionsPerTopic,
//			ReplicationFactor: Cfg.ReplicationFactor,
//			ConfigEntries: map[string]*string{
//				"cleanup.policy":                 strPtr("delete"),
//				"min.insync.replicas":            strPtr("1"),
//				"unclean.leader.election.enable": strPtr("false"),
//				"compression.type":               strPtr("producer"),
//			},
//		}
//		if err := admin.CreateTopic(t, td, false); err != nil {
//			if err == sarama.ErrTopicAlreadyExists {
//				log.Printf("[Topic] exists (race): %s", t)
//				continue
//			}
//			return fmt.Errorf("create topic %s: %w", t, err)
//		}
//		log.Printf("[Topic] created: %s (partitions=%d, rf=%d)", t, Cfg.PartitionsPerTopic, Cfg.ReplicationFactor)
//	}
//	return nil
//}

// EnsureTopicsWith  会：
// 1) 不存在就按 appCfg 创建；
// 2) 已存在且分区数 < 期望值时，执行 CreatePartitions 扩分区（Kafka 仅支持增加分区，不能减少）；
// 3) 写入基础 config（若你需要强一致改配置，可额外做 AlterConfigs/IncrementalAlterConfigs）。
//func EnsureTopicsWith(admin sarama.ClusterAdmin, topics []string, appCfg *AppConfig) error {
//	for _, t := range topics {
//		descs, err := admin.DescribeTopics([]string{t})
//		if err != nil {
//			return fmt.Errorf("describe topic %s: %w", t, err)
//		}
//		exists := len(descs) == 1 && errors.Is(sarama.ErrNoError, descs[0].Err)
//
//		// 期望配置
//		minISR := "1"
//		if appCfg.ReplicationFactor >= 3 {
//			minISR = "2" // 生产更安全：rf>=3 则至少 2
//		}
//
//		if !exists {
//			td := &sarama.TopicDetail{
//				NumPartitions:     appCfg.PartitionsPerTopic,
//				ReplicationFactor: appCfg.ReplicationFactor,
//				ConfigEntries: map[string]*string{
//					"cleanup.policy":                 strPtr("delete"), // 历史按需保留；也可用 compact
//					"min.insync.replicas":            strPtr(minISR),
//					"unclean.leader.election.enable": strPtr("false"),
//					"compression.type":               strPtr("producer"),
//				},
//			}
//			if err := admin.CreateTopic(t, td, false); err != nil {
//				// CreateTopic 可能返回 *sarama.TopicError 或通用 error
//				var te *sarama.TopicError
//				if errors.As(err, &te) && te.Err == sarama.ErrTopicAlreadyExists {
//					log.Printf("[Topic] exists (race): %s", t)
//					continue
//				}
//				// 兼容老写法
//				if errors.Is(err, sarama.ErrTopicAlreadyExists) {
//					log.Printf("[Topic] exists (race): %s", t)
//					continue
//				}
//				return fmt.Errorf("create topic %s: %w", t, err)
//			}
//			log.Printf("[Topic] created: %s (partitions=%d, rf=%d)", t, appCfg.PartitionsPerTopic, appCfg.ReplicationFactor)
//			continue
//		}
//
//		// 已存在：必要时扩分区
//		curParts := int32(len(descs[0].Partitions))
//		if appCfg.PartitionsPerTopic > curParts {
//			err := admin.CreatePartitions(t, appCfg.PartitionsPerTopic, nil, false)
//			if err != nil {
//				return fmt.Errorf("expand partitions %s from %d to %d: %w", t, curParts, appCfg.PartitionsPerTopic, err)
//			}
//			log.Printf("[Topic] partitions expanded: %s (%d -> %d)", t, curParts, appCfg.PartitionsPerTopic)
//		} else {
//			log.Printf("[Topic] exists: %s (partitions=%d, rf~=%d)", t, curParts, appCfg.ReplicationFactor)
//		}
//
//		// （可选）调整 topic 配置：
//		// 注意：不同 Kafka 版本对 AlterConfigs/IncrementalAlterConfigs 支持不同；
//		// 如需强一致下推 config，可在这里按 appCfg 走 IncrementalAlterConfigs。
//	}
//	return nil
//}

// ------------ 自动创建 topic（可选） ------------

// EnsureTopics 根据 MessageConfigs 创建缺失的 topics。
// - partitions: 若你没有在配置里给出，我们默认 3；可根据需要改成参数。
// - replication 取自每个 MessageHandlerConfig.ReplicationFactor；<=0 时退化为 1。
// - 已存在则忽略。
func EnsureTopics(ctx context.Context, app AppConfig) error {
	if len(app.Brokers) == 0 {
		return fmt.Errorf("no brokers provided")
	}
	// 聚合所有 config 各自的 KafkaVersion，取最小/最老以保证兼容；
	// 简化起见：如果多个版本不一致，这里用第一个非零版本，否则 V2_1_0_0。
	kver := sarama.V2_1_0_0
	for _, _ = range app.MessageConfigs {
		if app.KafkaVersion != (sarama.KafkaVersion{}) {
			kver = app.KafkaVersion
			break
		}
	}

	cfg := sarama.NewConfig()
	cfg.Version = kver
	cfg.Admin.Timeout = 15 * time.Second
	cfg.Net.DialTimeout = 10 * time.Second
	cfg.Net.ReadTimeout = 10 * time.Second
	cfg.Net.WriteTimeout = 10 * time.Second

	admin, err := sarama.NewClusterAdmin(app.Brokers, cfg)
	if err != nil {
		return fmt.Errorf("new cluster admin: %w", err)
	}
	defer func(admin sarama.ClusterAdmin) {
		err := admin.Close()
		if err != nil {
			logger.Errorf("close cluster admin: %w", err)
		}
	}(admin)

	// 获取已存在 topics，避免重复创建（也可以直接 Create 并忽略 TopicAlreadyExists）
	existing, err := admin.ListTopics()
	if err != nil {
		return fmt.Errorf("list topics: %w", err)
	}

	type topicPlan struct {
		name              string
		partitions        int32
		replicationFactor int16
		configEntries     map[string]*string
	}

	var plans []topicPlan

	for _, mc := range app.MessageConfigs {
		if !app.AutoCreateTopicsOnStart {
			continue
		}
		rep := int16(mc.ReplicationFactor)
		if rep <= 0 {
			rep = 1
		}
		// 你也可以把 partitions 加到 MessageHandlerConfig；这里先采用一个合理默认
		const defaultPartitions = int32(3)

		// 常见配置：保留 7 天、按大小与时间滚动、删除策略
		retentionMs := fmt.Sprintf("%d", 7*24*60*60*1000)
		segmentBytes := fmt.Sprintf("%d", 1<<30) // 1 GiB
		cfgs := map[string]*string{
			"cleanup.policy":                 ptr("delete"),
			"retention.ms":                   &retentionMs,
			"segment.bytes":                  &segmentBytes,
			"min.insync.replicas":            ptr("1"),
			"unclean.leader.election.enable": ptr("false"),
		}

		for _, pat := range mc.TopicPattern {
			for _, t := range expandPattern(string(pat), mc.TopicCount) {
				if _, ok := existing[t]; ok {
					continue
				}
				plans = append(plans, topicPlan{
					name:              t,
					partitions:        defaultPartitions,
					replicationFactor: rep,
					configEntries:     cfgs,
				})
			}
		}
	}

	for _, p := range plans {
		detail := &sarama.TopicDetail{
			NumPartitions:     p.partitions,
			ReplicationFactor: p.replicationFactor,
			ConfigEntries:     p.configEntries,
		}
		if err := admin.CreateTopic(p.name, detail, false); err != nil {
			// 幂等：忽略已存在错误
			if !isTopicExistsErr(err) {
				return fmt.Errorf("create topic %q: %w", p.name, err)
			}
		}
	}

	return nil
}

func isTopicExistsErr(err error) bool {
	// 兼容不同版本的错误字符串/类型
	if errors.Is(err, sarama.ErrTopicAlreadyExists) {
		return true
	}
	// 有的 broker 返回的是普通 error 文本
	return strings.Contains(strings.ToLower(err.Error()), "already exists")
}

func strPtr(s string) *string { return &s }
func ptr(s string) *string    { return &s }
