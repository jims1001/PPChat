package kafka

import (
	"fmt"

	"github.com/Shopify/sarama"
)

func SendMessage(topic, key, value string) error {
	if Producer == nil {
		return fmt.Errorf("producer not initialized")
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.StringEncoder(value),
	}

	partition, offset, err := Producer.SendMessage(msg)
	if err != nil {
		return err
	}

	fmt.Printf("send success！partical: %d, offset: %d\n", partition, offset)
	return nil
}
