package kafka

import (
	"strings"
	"time"

	"github.com/Shopify/sarama"
)

var (
	KafkaClient sarama.Client
	Producer    sarama.SyncProducer // send.go 使用
	SyncProd    sarama.SyncProducer // 兼容你之前的 sync.go
)

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
