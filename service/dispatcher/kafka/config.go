package kafka

import (
	pb "PProject/gen/message"
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
	MessageType           pb.MessageFrameData_Type
	SendTopicPattern      string
	ReceiveTopicPattern   string
	TopicCount            int
	ReplicationFactor     int
	ConsumerInitialOffset string
	IsConsumer            bool // 当前节点是否需要消费
	NodeId                string
}

func NewMessageHandlerConfig() *MessageHandlerConfig {
	return &MessageHandlerConfig{}
}

func (c *MessageHandlerConfig) genKeys(pattern string, count int, isNeedNode bool) []string {
	keys := make([]string, 0, count)

	if isNeedNode {
		pattern = fmt.Sprintf("%v_%v", c.NodeId, pattern)
	}

	for i := 0; i < count; i++ {
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

func (c *MessageHandlerConfig) Keys() []string {
	var out []string
	if c.IsConsumer {
		out = append(out, c.genKeys(c.SendTopicPattern, c.TopicCount, true)...)
	} else {
		out = append(out, c.genKeys(c.ReceiveTopicPattern, c.TopicCount, true)...)
	}

	return out
}

func (c *MessageHandlerConfig) SendTopicKeys() []string {
	return c.genKeys(c.SendTopicPattern, c.TopicCount, true)
}

func (c *MessageHandlerConfig) ReceiveTopicKeys(isNeedNode bool) []string {
	return c.genKeys(c.ReceiveTopicPattern, c.TopicCount, isNeedNode)
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

func (c *AppConfig) GetTopicKeys(messageType pb.MessageFrameData_Type) []string {

	for _, config := range c.MessageConfigs {
		if config.MessageType == messageType {
			return config.Keys()
		}
	}
	return nil
}

func (c *AppConfig) GetMessageHandlerConfig(messageType pb.MessageFrameData_Type) *MessageHandlerConfig {
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
	PartitionsPerTopic:      1, // 单机演示
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
