package kafka

import (
	"context"
	"log"
)

func BootConsumers(topics []string) (context.CancelFunc, error) {
	_, cancel := context.WithCancel(context.Background())
	if err := StartConsumerGroup(Cfg.Brokers, Cfg.GroupID, topics); err != nil {
		log.Printf("consumer group quit: %v", err)
	}
	log.Printf("[CG] started, topics=%v", topics)
	return cancel, nil
}
