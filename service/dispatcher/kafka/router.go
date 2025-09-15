package kafka

import (
	"fmt"
	"hash/crc32"
	"strings"
)

// GenTopics 生成 N 个大 Topic：im.shard-00, im.shard-01, ...
//func GenTopics() []string {
//	out := make([]string, 0, Cfg.TopicCount)
//	for i := 0; i < Cfg.TopicCount; i++ {
//		out = append(out, fmt.Sprintf(Cfg.TopicPattern, i))
//	}
//	return out
//}

//func GenCAckTopic() []string {
//	out := make([]string, 0, CAckCfg.TopicCount)
//	for i := 0; i < CAckCfg.TopicCount; i++ {
//		out = append(out, fmt.Sprintf(CAckCfg.TopicPattern, i))
//	}
//	return out
//}

//func GenTopicsWithPattern(cfg *AppConfig) []string {
//	out := make([]string, 0, cfg.TopicCount)
//	for i := 0; i < cfg.TopicCount; i++ {
//		out = append(out, fmt.Sprintf(cfg.TopicPattern, i))
//	}
//	return out
//}

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

// GenTopics 汇总所有 MessageConfigs 展开的 topic 名称（去重、保持插入顺序）
func GenTopics(app AppConfig) []string {
	seen := make(map[string]struct{})
	var out []string

	for _, mc := range app.MessageConfigs {
		for _, pat := range mc.TopicPattern {
			for _, t := range expandPattern(string(pat), mc.TopicCount) {
				if _, ok := seen[t]; ok {
					continue
				}
				seen[t] = struct{}{}
				out = append(out, t)
			}
		}
	}
	return out
}

// 支持三种写法：
// 1) 显式 printf: "topic-%02d"
// 2) 花括号占位:  "topic-{i}"
// 3) 无占位时:    "topic" => "topic-0"... (自动追加 "-%d")
func expandPattern(pattern string, n int) []string {
	if n <= 0 {
		// 兼容：当 n<=0 时按 1 扩展
		n = 1
	}
	hasPrintf := strings.Contains(pattern, "%d")
	hasBrace := strings.Contains(pattern, "{i}")

	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		switch {
		case hasPrintf:
			out = append(out, fmt.Sprintf(pattern, i))
		case hasBrace:
			out = append(out, strings.ReplaceAll(pattern, "{i}", fmt.Sprintf("%d", i)))
		default:
			out = append(out, fmt.Sprintf("%s-%d", pattern, i))
		}
	}
	return out
}
