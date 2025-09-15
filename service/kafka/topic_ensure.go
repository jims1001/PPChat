package kafka

import (
	"errors"
	"fmt"
	"log"

	"github.com/Shopify/sarama"
)

func EnsureTopics(admin sarama.ClusterAdmin, topics []string) error {
	for _, t := range topics {
		desc, err := admin.DescribeTopics([]string{t})
		if err == nil && len(desc) == 1 && desc[0].Err == sarama.ErrNoError {
			log.Printf("[Topic] exists: %s (partitions=%d)", t, len(desc[0].Partitions))
			continue
		}
		td := &sarama.TopicDetail{
			NumPartitions:     Cfg.PartitionsPerTopic,
			ReplicationFactor: Cfg.ReplicationFactor,
			ConfigEntries: map[string]*string{
				"cleanup.policy":                 strPtr("delete"),
				"min.insync.replicas":            strPtr("1"),
				"unclean.leader.election.enable": strPtr("false"),
				"compression.type":               strPtr("producer"),
			},
		}
		if err := admin.CreateTopic(t, td, false); err != nil {
			if err == sarama.ErrTopicAlreadyExists {
				log.Printf("[Topic] exists (race): %s", t)
				continue
			}
			return fmt.Errorf("create topic %s: %w", t, err)
		}
		log.Printf("[Topic] created: %s (partitions=%d, rf=%d)", t, Cfg.PartitionsPerTopic, Cfg.ReplicationFactor)
	}
	return nil
}

// EnsureTopicsWith  会：
// 1) 不存在就按 appCfg 创建；
// 2) 已存在且分区数 < 期望值时，执行 CreatePartitions 扩分区（Kafka 仅支持增加分区，不能减少）；
// 3) 写入基础 config（若你需要强一致改配置，可额外做 AlterConfigs/IncrementalAlterConfigs）。
func EnsureTopicsWith(admin sarama.ClusterAdmin, topics []string, appCfg *AppConfig) error {
	for _, t := range topics {
		descs, err := admin.DescribeTopics([]string{t})
		if err != nil {
			return fmt.Errorf("describe topic %s: %w", t, err)
		}
		exists := len(descs) == 1 && errors.Is(sarama.ErrNoError, descs[0].Err)

		// 期望配置
		minISR := "1"
		if appCfg.ReplicationFactor >= 3 {
			minISR = "2" // 生产更安全：rf>=3 则至少 2
		}

		if !exists {
			td := &sarama.TopicDetail{
				NumPartitions:     appCfg.PartitionsPerTopic,
				ReplicationFactor: appCfg.ReplicationFactor,
				ConfigEntries: map[string]*string{
					"cleanup.policy":                 strPtr("delete"), // 历史按需保留；也可用 compact
					"min.insync.replicas":            strPtr(minISR),
					"unclean.leader.election.enable": strPtr("false"),
					"compression.type":               strPtr("producer"),
				},
			}
			if err := admin.CreateTopic(t, td, false); err != nil {
				// CreateTopic 可能返回 *sarama.TopicError 或通用 error
				var te *sarama.TopicError
				if errors.As(err, &te) && te.Err == sarama.ErrTopicAlreadyExists {
					log.Printf("[Topic] exists (race): %s", t)
					continue
				}
				// 兼容老写法
				if errors.Is(err, sarama.ErrTopicAlreadyExists) {
					log.Printf("[Topic] exists (race): %s", t)
					continue
				}
				return fmt.Errorf("create topic %s: %w", t, err)
			}
			log.Printf("[Topic] created: %s (partitions=%d, rf=%d)", t, appCfg.PartitionsPerTopic, appCfg.ReplicationFactor)
			continue
		}

		// 已存在：必要时扩分区
		curParts := int32(len(descs[0].Partitions))
		if appCfg.PartitionsPerTopic > curParts {
			err := admin.CreatePartitions(t, appCfg.PartitionsPerTopic, nil, false)
			if err != nil {
				return fmt.Errorf("expand partitions %s from %d to %d: %w", t, curParts, appCfg.PartitionsPerTopic, err)
			}
			log.Printf("[Topic] partitions expanded: %s (%d -> %d)", t, curParts, appCfg.PartitionsPerTopic)
		} else {
			log.Printf("[Topic] exists: %s (partitions=%d, rf~=%d)", t, curParts, appCfg.ReplicationFactor)
		}

		// （可选）调整 topic 配置：
		// 注意：不同 Kafka 版本对 AlterConfigs/IncrementalAlterConfigs 支持不同；
		// 如需强一致下推 config，可在这里按 appCfg 走 IncrementalAlterConfigs。
	}
	return nil
}

func strPtr(s string) *string { return &s }
