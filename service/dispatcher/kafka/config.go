package kafka

import (
	"fmt"
	"strings"

	"github.com/Shopify/sarama"
)

// MessageHandlerConfig 消息处理 通过配置不同的key进行处理

// MessageHandlerConfig MessageTypeData 普通的消息 1
// MessageHandlerConfig MessageTypeCAck 客户端回执消息 2

const MessageTypeDataSender = 1 //普通的消息 1
const MessageTypeDataReceiver = 2
const MessageTypeCAck = 2 //客户端回执消息
type MessageHandlerConfig struct {
	Handler               MessageHandler
	MessageType           int
	TopicPattern          string
	TopicCount            int
	ReplicationFactor     int
	ConsumerInitialOffset string
	IsConsumer            bool // 当前节点是否需要消费
}

func NewMessageHandlerConfig() *MessageHandlerConfig {
	return &MessageHandlerConfig{}
}

func (c *MessageHandlerConfig) Keys() []string {

	hasPrintf := strings.Contains(c.TopicPattern, "%d")
	hasBrace := strings.Contains(c.TopicPattern, "{i}")

	out := make([]string, 0, c.TopicCount)
	for i := 0; i < c.TopicCount; i++ {
		switch {
		case hasPrintf:
			out = append(out, fmt.Sprintf(c.TopicPattern, i))
		case hasBrace:
			out = append(out, strings.ReplaceAll(c.TopicPattern, "{i}", fmt.Sprintf("%d", i)))
		default:
			out = append(out, fmt.Sprintf("%s-%d", c.TopicPattern, i))
		}
	}
	return out
}

// AppConfig In-code 配置（不读 YAML）
type AppConfig struct {
	Brokers                 []string
	GroupID                 string
	MessageConfigs          []MessageHandlerConfig // 消息配置
	ProducerCompression     string
	ConsumerInitialOffset   string
	KafkaVersion            sarama.KafkaVersion
	ProducerRetries         int
	AutoCreateTopicsOnStart bool
	ReplicationFactor       int
	PartitionsPerTopic      int
}

func NewAppConfig() *AppConfig {
	return &AppConfig{}
}

func (c *AppConfig) GetTopicKeys(messageType int) []string {

	for _, config := range c.MessageConfigs {
		if config.MessageType == messageType {
			return config.Keys()
		}
	}
	return nil
}

func (c *AppConfig) GetMessageHandlerConfig(messageType int) *MessageHandlerConfig {
	for _, config := range c.MessageConfigs {
		if config.MessageType == messageType {
			return &config
		}
	}
	return nil
}

func (c *AppConfig) GetAllTopicKeys() []string {
	var out []string
	for _, config := range c.MessageConfigs {

		// 如果不需要消费 就需要监听
		if !config.IsConsumer {
			continue
		}

		keys := config.Keys()
		out = append(out, keys...)
	}
	return out
}

//TopicPattern            string // 例如 "im.shard-%02d"
//TopicCount              int    // 32/64/128…
//PartitionsPerTopic      int32  // Demo: 8；生产：512~1024
//ReplicationFactor       int16  // 单机=1；生产=3
//ProducerRetries         int
//ProducerCompression     string // none/snappy/lz4/zstd
//ConsumerInitialOffset   string // newest/oldest
//KafkaVersion            sarama.KafkaVersion
//AutoCreateTopicsOnStart bool
//Handlers                []MessageHandler

// Cfg 默认配置（可直接改）
var Cfg = AppConfig{
	Brokers: []string{"127.0.0.1:9092"},
	//TopicPattern:            "im.shard-%02d",
	//TopicCount:              2, // 改成 64/128 即可
	PartitionsPerTopic:      2, // 单机演示
	ReplicationFactor:       1, // 单机演示
	ProducerRetries:         5,
	ProducerCompression:     "snappy",
	ConsumerInitialOffset:   "newest",
	KafkaVersion:            sarama.V2_1_0_0,
	AutoCreateTopicsOnStart: true,
}

var CAckCfg = AppConfig{
	Brokers: []string{"127.0.0.1:9092"},
	GroupID: "im-app-consumer-1",
	//TopicPattern:            "im.shard-client_ack-%02d",
	//TopicCount:              2, // 改成 64/128 即可
	//PartitionsPerTopic:      8, // 单机演示
	//ReplicationFactor:       1, // 单机演示
	//ProducerRetries:         5,
	//ProducerCompression:     "snappy",
	//ConsumerInitialOffset:   "newest",
	//KafkaVersion:            sarama.V2_1_0_0,
	//AutoCreateTopicsOnStart: true,
}
