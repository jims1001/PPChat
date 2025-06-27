package kafka

import (
	"github.com/Shopify/sarama"
)

var KafkaClient sarama.Client

func InitKafkaClient(brokers []string) error {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 3
	config.Producer.Partitioner = sarama.NewRandomPartitioner

	client, err := sarama.NewClient(brokers, config)
	if err != nil {
		return err
	}
	KafkaClient = client
	return nil
}
