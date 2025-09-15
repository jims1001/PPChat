package kafka

import (
	"fmt"
	"hash/crc32"
)

// GenTopics 生成 N 个大 Topic：im.shard-00, im.shard-01, ...
func GenTopics() []string {
	out := make([]string, 0, Cfg.TopicCount)
	for i := 0; i < Cfg.TopicCount; i++ {
		out = append(out, fmt.Sprintf(Cfg.TopicPattern, i))
	}
	return out
}

func GenCAckTopic() []string {
	out := make([]string, 0, CAckCfg.TopicCount)
	for i := 0; i < CAckCfg.TopicCount; i++ {
		out = append(out, fmt.Sprintf(CAckCfg.TopicPattern, i))
	}
	return out
}

func GenTopicsWithPattern(cfg *AppConfig) []string {
	out := make([]string, 0, cfg.TopicCount)
	for i := 0; i < cfg.TopicCount; i++ {
		out = append(out, fmt.Sprintf(cfg.TopicPattern, i))
	}
	return out
}

// SelectTopicByUser  同一 userId 永远命中同一个大 Topic
func SelectTopicByUser(userId string, topics []string) string {
	if len(topics) == 0 {
		return ""
	}
	h := crc32.ChecksumIEEE([]byte(userId))
	idx := int(h % uint32(len(topics)))
	return topics[idx]
}

func SelectCAckTopicByUser(userId string, topics []string) string {
	if len(topics) == 0 {
		return ""
	}
	h := crc32.ChecksumIEEE([]byte(userId))
	idx := int(h % uint32(len(topics)))
	return topics[idx]
}
