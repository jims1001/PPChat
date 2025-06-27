package kafka

import "github.com/Shopify/sarama"

var SyncProd sarama.SyncProducer

func InitSyncProducerFromClient() error {
	p, err := sarama.NewSyncProducerFromClient(KafkaClient)
	if err != nil {
		return err
	}
	SyncProd = p
	return nil
}

func SendSync(topic, value string) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(value),
	}
	_, _, err := SyncProd.SendMessage(msg)
	return err
}
