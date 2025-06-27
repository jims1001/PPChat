package kafka

import (
	"github.com/Shopify/sarama"
	"log"
)

var AsyncProd sarama.AsyncProducer

func InitAsyncProducerFromClient() error {
	p, err := sarama.NewAsyncProducerFromClient(KafkaClient)
	if err != nil {
		return err
	}
	AsyncProd = p

	go func() {
		for {
			select {
			case msg := <-AsyncProd.Successes():
				log.Printf("Async message sent to topic=%s partition=%d offset=%d", msg.Topic, msg.Partition, msg.Offset)
			case err := <-AsyncProd.Errors():
				log.Printf("Async message error: %v", err)
			}
		}
	}()

	return nil
}

func SendAsync(topic, value string) {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(value),
	}
	AsyncProd.Input() <- msg
}
