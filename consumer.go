package main

import (
	"PProject/service/kafka"
	"log"
)

func main() {

	kafka.RegisterHandler("test-topic", handleTestTopic)
	kafka.RegisterHandler("log-topic", handleLogTopic)

	// 启动消费者
	err := kafka.StartConsumerGroup([]string{"localhost:9092"}, "my-group", []string{
		"test-topic",
		"log-topic",
	})
	if err != nil {
		log.Fatalf("Kafka consumer failed: %v", err)
	}
}

func handleTestTopic(topic string, key, value []byte) error {
	log.Printf("[TestTopic] key=%s, value=%s", key, value)
	return nil
}

func handleLogTopic(topic string, key, value []byte) error {
	log.Printf("[LogTopic] key=%s, value=%s", key, value)
	return nil
}
