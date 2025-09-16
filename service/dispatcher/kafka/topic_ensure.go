package kafka

import (
	"PProject/logger"
	"context"
	"errors"
	"fmt"
	"sort"
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

func EnsureTopics(ctx context.Context, app AppConfig) error {
	if len(app.Brokers) == 0 {
		return fmt.Errorf("no brokers provided")
	}

	// 1) Kafka 版本：优先用 App 级；为空回退默认
	kver := app.KafkaVersion
	if kver == (sarama.KafkaVersion{}) {
		kver = sarama.V2_1_0_0
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
	defer func() {
		if e := admin.Close(); e != nil {
			logger.Errorf("close cluster admin: %v", e)
		}
	}()

	// 2) 获取已存在 topics：key 即为 topic 名称
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

	const defaultPartitions = int32(3)

	// 用 map 聚合，避免跨 mc 重复
	planMap := make(map[string]topicPlan, 64)

	// 通用默认配置
	baseRetentionMs := fmt.Sprintf("%d", 7*24*60*60*1000) // 7天
	segmentBytes := fmt.Sprintf("%d", 1<<30)              // 1 GiB

	for _, mc := range app.MessageConfigs {
		if !app.AutoCreateTopicsOnStart {
			// 两侧都没打开自动创建，就跳过
			continue
		}

		rep := int16(mc.ReplicationFactor)
		if rep <= 0 {
			rep = 1
		}
		// min.insync.replicas 跟随副本数：至少1，尽量设置为rep-1
		minISR := "1"
		if rep > 1 {
			minISR = fmt.Sprintf("%d", rep-1)
		}

		cfgs := map[string]*string{
			"cleanup.policy":                 ptr("delete"),
			"retention.ms":                   &baseRetentionMs,
			"segment.bytes":                  &segmentBytes,
			"min.insync.replicas":            &minISR,
			"unclean.leader.election.enable": ptr("false"),
		}

		// 统一展开 -> candidate 名单
		candidates := make([]string, 0, mc.TopicCount*2)
		candidates = append(candidates, expandPatterns([]string{mc.SendTopicPattern}, mc.TopicCount)...)
		candidates = append(candidates, expandPatterns([]string{mc.ReceiveTopicPattern}, mc.TopicCount)...)

		// 分区数支持 mc 自定义（若你在 MessageHandlerConfig 里加了 Partitions 字段）
		partitions := defaultPartitions
		if v := getPartitionsFromMC(mc); v > 0 {
			partitions = v
		}

		for _, t := range candidates {
			if _, ok := existing[t]; ok {
				// 已存在，不再创建；但仍可考虑后续校验/对齐配置（如需）
				continue
			}
			if _, dup := planMap[t]; dup {
				continue
			}
			planMap[t] = topicPlan{
				name:              t,
				partitions:        partitions,
				replicationFactor: rep,
				configEntries:     cfgs,
			}
		}

		// —— 注册 handler —— //
		switch {
		case hasMethod(mc, "SendTopicKeys") && hasMethod(mc, "ReceiveTopicKeys"):
			// 如果已经实现了前面你拆分出的两个方法，优先使用
			if sendKeys := safeSendKeys(mc); len(sendKeys) > 0 {
				RegisterDefaultHandlers(sendKeys, mc.Handler)
			}
			if recvKeys := safeRecvKeys(mc); len(recvKeys) > 0 {
				RegisterDefaultHandlers(recvKeys, mc.Handler)
			}
		default:
			// 兼容老接口：Keys() 合并返回
			RegisterDefaultHandlers(mc.Keys(), mc.Handler)
		}
	}

	// 3) 排序后批量创建（幂等）
	if len(planMap) == 0 {
		return nil
	}
	names := make([]string, 0, len(planMap))
	for k := range planMap {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, name := range names {
		p := planMap[name]
		detail := &sarama.TopicDetail{
			NumPartitions:     p.partitions,
			ReplicationFactor: p.replicationFactor,
			ConfigEntries:     p.configEntries,
		}
		if err := admin.CreateTopic(p.name, detail, false); err != nil {
			if !isTopicExistsErr(err) {
				return fmt.Errorf("create topic %q: %w", p.name, err)
			}
		}
	}

	return nil
}

// —— helpers ——

// 展开一组 pattern（支持 "%d" / "{i}" / 默认后缀 -i）
func expandPatterns(patterns []string, count int) []string {
	out := make([]string, 0, len(patterns)*max(1, count))
	for _, pat := range patterns {
		out = append(out, expandPattern(pat, count)...)
	}
	return out
}

func expandPattern(pattern string, count int) []string {
	keys := make([]string, 0, max(1, count))
	for i := 0; i < max(1, count); i++ {
		switch {
		case strings.Contains(pattern, "%d"):
			keys = append(keys, fmt.Sprintf(pattern, i))
		case strings.Contains(pattern, "{i}"):
			keys = append(keys, strings.ReplaceAll(pattern, "{i}", fmt.Sprintf("%d", i)))
		default:
			keys = append(keys, fmt.Sprintf("%s-%d", pattern, i))
		}
	}
	return keys
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func ptr[T any](v T) *T { return &v }

// 若你在 MessageHandlerConfig 中新增了 Partitions 字段，这里读取；没有就返回 0
func getPartitionsFromMC(mc MessageHandlerConfig) int32 {
	// 伪代码：按你的结构体自行调整
	// if mc.Partitions > 0 { return int32(mc.Partitions) }
	return 0
}

// 下面两个是“安全调用”占位；如果你已经实现了 SendTopicKeys()/ReceiveTopicKeys() 就直接调；
// 若还没有，就可以删掉 hasMethod/safeXxx 相关代码，直接用 Keys()。
func hasMethod(any interface{}, name string) bool   { return true } // 简化：根据你实际情况删掉
func safeSendKeys(mc MessageHandlerConfig) []string { return mc.SendTopicKeys() }
func safeRecvKeys(mc MessageHandlerConfig) []string { return mc.ReceiveTopicKeys(true) }

func isTopicExistsErr(err error) bool {
	// 兼容不同版本的错误字符串/类型
	if errors.Is(err, sarama.ErrTopicAlreadyExists) {
		return true
	}
	// 有的 broker 返回的是普通 error 文本
	return strings.Contains(strings.ToLower(err.Error()), "already exists")
}
