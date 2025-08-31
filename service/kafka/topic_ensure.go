package kafka

import (
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

func strPtr(s string) *string { return &s }
