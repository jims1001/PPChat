package kafka

import (
	"log"
	"sync"
	"testing"
	"time"
)

func handleTestTopic(topic string, key, value []byte) error {
	log.Printf("[TestTopic] key=%s, value=%s", key, value)
	return nil
}

func TestConnectKafka(t *testing.T) {

	brokers := []string{"localhost:9092"}

	if err := InitKafkaClient(brokers); err != nil {
		log.Fatal("InitKafkaClient failed:", err)
	}

	// 获取 broker 列表作为验证
	brokerCount := len(KafkaClient.Brokers())
	if brokerCount == 0 {
		t.Fatalf("No brokers found in cluster")
	}

	t.Logf("Successfully connected to Kafka. Broker count: %d", brokerCount)
}

func TestSendKafkaMessage(t *testing.T) {

	brokers := []string{"localhost:9092"}
	topic := "test-topic"
	message := "send kafka system message"
	// 初始化 Kafka client
	if err := InitKafkaClient(brokers); err != nil {
		t.Fatalf("InitKafkaClient failed: %v", err)
	}

	// 初始化同步生产者
	if err := InitSyncProducerFromClient(); err != nil {
		t.Fatalf("InitSyncProducer failed: %v", err)
	}

	// 发送同步消息
	if err := SendSync(topic, message); err != nil {
		t.Errorf("SendSync failed: %v", err)
	} else {
		t.Logf("Message sent successfully to topic %s: %s", topic, message)
	}

}

func TestKafkaConsumerGroup(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)

	// handler for test-topic
	RegisterHandler("test-topic", handleTestTopic)

	go func() {
		err := StartConsumerGroup(
			[]string{"localhost:9092"},
			"my-test-group",
			[]string{"test-topic", "log-topic"},
		)
		if err != nil {
			t.Errorf("Kafka consumer group error: %v", err)
		}
	}()

	time.Sleep(10 * time.Second)
	TestSendKafkaMessage(t)
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()

	select {
	case <-waitChan:
		t.Log("All messages received and processed")
	case <-time.After(10 * time.Second):
		t.Log("Timeout waiting for message handlers")
	}
}
