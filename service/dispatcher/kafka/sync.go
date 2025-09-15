package kafka

import "github.com/Shopify/sarama"

var SyncProdLegacy sarama.SyncProducer // 避免与 Producer/SyncProd 冲突

func SendSync(topic, value string) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(value),
	}
	_, _, err := SyncProd.SendMessage(msg) // 使用 Producer/SyncProd 任一均可
	return err
}
