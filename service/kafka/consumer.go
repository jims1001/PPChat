package kafka

import (
	"context"
	"log"

	"github.com/Shopify/sarama"
)

type ConsumerGroupHandler struct{}

func (h *ConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	log.Println("Consumer group setup")
	return nil
}

func (h *ConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	log.Println("Consumer group cleanup")
	return nil
}

func (h *ConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("Received message | topic=%s | partition=%d | offset=%d", msg.Topic, msg.Partition, msg.Offset)

		handler, err := GetHandler(msg.Topic)
		if err != nil {
			log.Printf("No handler for topic %s: %v", msg.Topic, err)
		} else {
			if err := handler(msg.Topic, msg.Key, msg.Value); err != nil {
				log.Printf("Handler error for topic %s: %v", msg.Topic, err)
			}
		}

		session.MarkMessage(msg, "")
	}
	return nil
}

func StartConsumerGroup(brokers []string, groupID string, topics []string) error {
	config := sarama.NewConfig()
	config.Version = sarama.V2_1_0_0
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Return.Errors = true

	group, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return err
	}

	go func() {
		for err := range group.Errors() {
			log.Printf("Consumer group error: %v", err)
		}
	}()

	ctx := context.Background()
	handler := &ConsumerGroupHandler{}
	for {
		if err := group.Consume(ctx, topics, handler); err != nil {
			log.Printf("Consume error: %v", err)
		}
	}
}
