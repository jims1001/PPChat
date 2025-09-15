package main

func main() {
	//brokers := []string{"localhost:9092"}
	//
	//if err := kafka.InitKafkaClient(brokers); err != nil {
	//	log.Fatal("InitKafkaClient failed:", err)
	//}
	//defer func(KafkaClient sarama.Client) {
	//	err := KafkaClient.Close()
	//	if err != nil {
	//
	//	}
	//}(kafka.KafkaClient)
	//
	//if err := kafka.InitSyncProducerFromClient(); err != nil {
	//	log.Fatal("InitSyncProducer failed:", err)
	//}
	//defer func(SyncProd sarama.SyncProducer) {
	//	err := SyncProd.Close()
	//	if err != nil {
	//
	//	}
	//}(kafka.SyncProd)
	//
	//if err := kafka.InitAsyncProducerFromClient(); err != nil {
	//	log.Fatal("InitAsyncProducer failed:", err)
	//}
	//defer func(AsyncProd sarama.AsyncProducer) {
	//	err := AsyncProd.Close()
	//	if err != nil {
	//
	//	}
	//}(kafka.AsyncProd)
	//
	//err := kafka.SendSync("test-topic", "hello from sync")
	//if err != nil {
	//	log.Println("Sync send error:", err)
	//}
	//
	//kafka.SendAsync("test-topic", "hello from async")

}
