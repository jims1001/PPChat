package main

import (
	"PProject/logger"
	"PProject/service/kafka"
)

func main() {

	kafka.RegisterHandler("test-topic", handleTestTopic)
	kafka.RegisterHandler("log-topic", handleLogTopic)

	// Start consumer
	err := kafka.StartConsumerGroup([]string{"localhost:9092"}, "my-group", []string{
		"test-topic",
		"log-topic",
	})
	if err != nil {
		logger.Errorf("Kafka consumer failed: %v", err)
	}
}

func handleTestTopic(topic string, key, value []byte) error {
	logger.Infof("[TestTopic] key=%s, value=%s", key, value)
	return nil
}

func handleLogTopic(topic string, key, value []byte) error {
	logger.Infof("[LogTopic] key=%s, value=%s", key, value)
	return nil
}
