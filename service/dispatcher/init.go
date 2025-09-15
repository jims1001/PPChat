package dispatcher

//func InitKafka(handler kafka.MessageHandler) {
//	go func() {
//		ctx, cancel := context.WithCancel(context.Background())
//		defer cancel()
//
//		// 1) 生成所有大 Topic 名称
//		topics := kafka.GenTopics()
//		allTopics := append(kafka.GenTopics(), kafka.GenCAckTopic()...)
//		logger.Infof("[Kafka] allTopics=%v", allTopics)
//
//		// 2) 启动前（可选）创建 Topic
//		if kafka.Cfg.AutoCreateTopicsOnStart {
//			adminCfg := kafka.BuildBaseConfig()
//			admin, err := sarama.NewClusterAdmin(kafka.Cfg.Brokers, adminCfg)
//			if err != nil {
//				logger.Infof("[Kafka][ERR] create admin: %v", err)
//				return
//			}
//			if err := kafka.EnsureTopics(admin, allTopics); err != nil {
//				logger.Infof("[Kafka][ERR] ensure allTopics: %v", err)
//				_ = admin.Close()
//				return
//			}
//			_ = admin.Close()
//		}
//
//		// 3) 初始化 Client & Producer
//		if err := kafka.InitKafkaClient(); err != nil {
//			logger.Infof("[Kafka][ERR] init client: %v", err)
//			return
//		}
//		if err := kafka.InitSyncProducerFromClient(); err != nil {
//			logger.Infof("[Kafka][ERR] init producer: %v", err)
//			return
//		}
//
//		// 4) 注册默认 handler
//		kafka.RegisterDefaultHandlers(topics, msg.HandlerTopicMessage)
//		kafka.RegisterDefaultHandlers(kafka.GenCAckTopic(), msg.HandlerCAckTopicMessage)
//
//		// 5) 启动 ConsumerGroup
//		_, err := kafka.BootConsumers(allTopics)
//		if err != nil {
//			return
//		}
//
//		// 6) 阻塞直到 ctx.Done()
//		select {
//		case <-ctx.Done():
//			logger.Infof("[Kafka] context done, shutting down")
//		}
//	}()
//}
