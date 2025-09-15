package message

import (
	"PProject/logger"
	ka "PProject/service/kafka"

	"github.com/Shopify/sarama"
)

func MessageProducerHandler(topic, key string, value []byte) error {
	logger.Infof("topic key value is %s", string(key))
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder([]byte(key)), // ★ 用 userId 作为 Key（HashPartitioner 生效）
		Value: sarama.ByteEncoder(value),
	}

	partition, offset, err := ka.Producer.SendMessage(msg)
	if err != nil {
		logger.Errorf("send message fail, %s", err)
	}

	logger.Infof("send message success, partition is %d offset:%d", partition, offset)
	return nil
}
