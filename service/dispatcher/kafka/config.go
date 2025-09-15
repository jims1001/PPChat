package kafka

import (
	"github.com/Shopify/sarama"
)

// MessageHandlerConfig 消息处理 通过配置不同的key进行处理

type MessageHandlerConfig struct {
	Handler                 MessageHandler
	TopicPattern            []string
	TopicCount              int
	ReplicationFactor       int
	KafkaVersion            sarama.KafkaVersion
	AutoCreateTopicsOnStart bool
	ConsumerInitialOffset   string
}

// AppConfig In-code 配置（不读 YAML）
type AppConfig struct {
	Brokers                 []string
	GroupID                 string
	TopicPattern            string // 例如 "im.shard-%02d"
	TopicCount              int    // 32/64/128…
	PartitionsPerTopic      int32  // Demo: 8；生产：512~1024
	ReplicationFactor       int16  // 单机=1；生产=3
	ProducerRetries         int
	ProducerCompression     string // none/snappy/lz4/zstd
	ConsumerInitialOffset   string // newest/oldest
	KafkaVersion            sarama.KafkaVersion
	AutoCreateTopicsOnStart bool
	Handlers                []MessageHandler
}

// Cfg 默认配置（可直接改）
var Cfg = AppConfig{
	Brokers:                 []string{"127.0.0.1:9092"},
	GroupID:                 "global.GlobalConfig.GroupId",
	TopicPattern:            "im.shard-%02d",
	TopicCount:              2, // 改成 64/128 即可
	PartitionsPerTopic:      8, // 单机演示
	ReplicationFactor:       1, // 单机演示
	ProducerRetries:         5,
	ProducerCompression:     "snappy",
	ConsumerInitialOffset:   "newest",
	KafkaVersion:            sarama.V2_1_0_0,
	AutoCreateTopicsOnStart: true,
}

var CAckCfg = AppConfig{
	Brokers:                 []string{"127.0.0.1:9092"},
	GroupID:                 "im-app-consumer-1",
	TopicPattern:            "im.shard-client_ack-%02d",
	TopicCount:              2, // 改成 64/128 即可
	PartitionsPerTopic:      8, // 单机演示
	ReplicationFactor:       1, // 单机演示
	ProducerRetries:         5,
	ProducerCompression:     "snappy",
	ConsumerInitialOffset:   "newest",
	KafkaVersion:            sarama.V2_1_0_0,
	AutoCreateTopicsOnStart: true,
}
