package kafka

import (
	"fmt"
	"strings"
	"time"

	"github.com/Shopify/sarama"
)

var (
	KafkaClient sarama.Client
	Producer    sarama.SyncProducer // send.go 使用
	SyncProd    sarama.SyncProducer // 兼容你之前的 sync.go
)

type FixedPartitioner struct{}

func (p *FixedPartitioner) Partition(msg *sarama.ProducerMessage, numPartitions int32) (int32, error) {
	return 0, nil // 永远选分区 0
}

func (p *FixedPartitioner) RequiresConsistency() bool { return true }

func BuildBaseConfigWith(appCfg *AppConfig) *sarama.Config {
	cfg := sarama.NewConfig()

	// Kafka 版本（给个兜底，避免零值触发 sarama 校验失败）
	if appCfg.KafkaVersion == (sarama.KafkaVersion{}) {
		cfg.Version = sarama.V2_8_0_0
	} else {
		cfg.Version = appCfg.KafkaVersion
	}

	// Producer
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll

	retries := appCfg.ProducerRetries
	if retries <= 0 {
		retries = 1
	}
	cfg.Producer.Retry.Max = retries

	// ★ 关键：Key 控制分区（与旧实现保持一致）
	cfg.Producer.Partitioner = sarama.NewHashPartitioner

	switch strings.ToLower(appCfg.ProducerCompression) {
	case "snappy":
		cfg.Producer.Compression = sarama.CompressionSnappy
	case "lz4":
		cfg.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		cfg.Producer.Compression = sarama.CompressionZSTD
	default:
		cfg.Producer.Compression = sarama.CompressionNone
	}

	// Consumer
	switch strings.ToLower(appCfg.ConsumerInitialOffset) {
	case "oldest":
		cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	default:
		cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	}
	cfg.Consumer.Return.Errors = true

	// Net（保持与你原来的超时一致；如需可配再把字段加进 AppConfig）
	cfg.Net.DialTimeout = 10 * time.Second
	cfg.Net.ReadTimeout = 30 * time.Second
	cfg.Net.WriteTimeout = 30 * time.Second

	// 最后校验，尽早暴露配置问题（可选）
	_ = cfg.Validate() // 忽略错误也行；若想强校验可改为返回 (*sarama.Config, error)

	return cfg
}

func BuildBaseConfig() *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Version = Cfg.KafkaVersion

	// Producer
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	if Cfg.ProducerRetries <= 0 {
		Cfg.ProducerRetries = 1
	}
	cfg.Producer.Retry.Max = Cfg.ProducerRetries

	cfg.Producer.Partitioner = sarama.NewHashPartitioner // ★ 关键：Key 控制分区
	switch strings.ToLower(Cfg.ProducerCompression) {
	case "snappy":
		cfg.Producer.Compression = sarama.CompressionSnappy
	case "lz4":
		cfg.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		cfg.Producer.Compression = sarama.CompressionZSTD
	default:
		cfg.Producer.Compression = sarama.CompressionNone
	}

	// Consumer（供 consumer.go 使用）
	switch strings.ToLower(Cfg.ConsumerInitialOffset) {
	case "oldest":
		cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	default:
		cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	}
	cfg.Consumer.Return.Errors = true

	// Net
	cfg.Net.DialTimeout = 10 * time.Second
	cfg.Net.ReadTimeout = 30 * time.Second
	cfg.Net.WriteTimeout = 30 * time.Second
	return cfg
}

// InitKafkaClient == 你之前版本的 InitKafkaClient 的“配置直接内置版” ==
func InitKafkaClient() error {
	cfg := BuildBaseConfig()
	c, err := sarama.NewClient(Cfg.Brokers, cfg)
	if err != nil {
		return err
	}
	KafkaClient = c
	return nil
}

func InitKafkaClientWith(appCfg *AppConfig) (sarama.Client, error) {
	if len(appCfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers is empty")
	}
	cfg := BuildBaseConfigWith(appCfg)
	if err := cfg.Validate(); err != nil { // 早期发现版本/参数问题
		return nil, fmt.Errorf("sarama config validate: %w", err)
	}
	c, err := sarama.NewClient(appCfg.Brokers, cfg)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// InitSyncProducerFromClient == 同步生产者（沿用你之前的思路，但赋值 Producer/SyncProd） ==
func InitSyncProducerFromClient() error {
	p, err := sarama.NewSyncProducerFromClient(KafkaClient)
	if err != nil {
		return err
	}
	Producer = p
	SyncProd = p
	return nil
}
