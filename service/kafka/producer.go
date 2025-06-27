package kafka

import (
	"github.com/Shopify/sarama"
	"log"
	"sync"
)

var Producer sarama.SyncProducer
var once sync.Once
var initErr error

func Init(brokers []string) error {
	once.Do(func() {
		config := sarama.NewConfig()
		config.Producer.Return.Successes = true
		config.Producer.RequiredAcks = sarama.WaitForAll
		config.Producer.Retry.Max = 5

		Producer, initErr = sarama.NewSyncProducer(brokers, config)
	})
	return initErr
}

func Close() {
	if Producer != nil {
		if err := Producer.Close(); err != nil {
			log.Printf("close producer fail: %v", err)
		}
	}
}
