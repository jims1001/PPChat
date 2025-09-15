package main

import (
	"PProject/logger"
	kafka2 "PProject/service/dispatcher/kafka"
)

func main() {

	kafka2.RegisterHandler("test-topic", handleTestTopic)
	kafka2.RegisterHandler("log-topic", handleLogTopic)

	// Start consumer
	err := kafka2.StartConsumerGroup([]string{"localhost:9092"}, "my-group", []string{
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
