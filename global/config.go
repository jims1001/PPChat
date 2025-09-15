package global

import (
	"PProject/data/database/mgo/mongoutil"
	mid "PProject/middleware"
	msg "PProject/module/message"
	ka "PProject/service/kafka"
	mgoSrv "PProject/service/mgo"
	redis "PProject/service/storage/redis"
	ids "PProject/tools/ids"
	"context"

	sarama "github.com/Shopify/sarama"
	"github.com/golang/glog"
)

func ConfigAll() {
	ConfigIds()
	ConfigRedis()
	ConfigMgo()
	//ConfigKafka()

}

func ConfigIds() {
	ids.SetNodeID(100)
}

func GetJwtSecret() []byte {

	b := []byte("mN9b1f8zPq+W2xjX/45sKcVd0TfyoG+3Hp5Z8q9Rj1o=")
	return b
}

func ConfigRedis() {
	config := redis.Config{
		Addr: "127.0.0.1:7001", Password: "password", DB: 0,
	}
	err := redis.InitRedis(config)
	if err != nil {
		return
	}
}

func ConfigMgo() {

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cfg := &mongoutil.Config{
			Uri:         "mongodb://localhost:27017",
			Database:    "agentChat",
			MaxPoolSize: 20,
			Username:    "root",
			Password:    "example",
			MaxRetry:    3, // 这里不用了，StartAsync 里我们自己做了指数退避
		}

		// 1) 异步启动
		mgoSrv.StartAsync(ctx, cfg)
		err := mgoSrv.WaitReady(ctx, mgoSrv.Manager())
		if err != nil {
			return
		}
		select {
		case <-ctx.Done():
		}
	}()

}

// ConfigKafka 在后台 goroutine 中启动 Kafka Client / Producer / Consumer
func ConfigKafka(handler ka.MessageHandler) {
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 1) 生成所有大 Topic 名称
		topics := ka.GenTopics()
		allTopics := append(ka.GenTopics(), ka.GenCAckTopic()...)
		glog.Infof("[Kafka] allTopics=%v", allTopics)

		// 2) 启动前（可选）创建 Topic
		if ka.Cfg.AutoCreateTopicsOnStart {
			adminCfg := ka.BuildBaseConfig()
			admin, err := sarama.NewClusterAdmin(ka.Cfg.Brokers, adminCfg)
			if err != nil {
				glog.Infof("[Kafka][ERR] create admin: %v", err)
				return
			}
			if err := ka.EnsureTopics(admin, allTopics); err != nil {
				glog.Infof("[Kafka][ERR] ensure allTopics: %v", err)
				_ = admin.Close()
				return
			}
			_ = admin.Close()
		}

		// 3) 初始化 Client & Producer
		if err := ka.InitKafkaClient(); err != nil {
			glog.Infof("[Kafka][ERR] init client: %v", err)
			return
		}
		if err := ka.InitSyncProducerFromClient(); err != nil {
			glog.Infof("[Kafka][ERR] init producer: %v", err)
			return
		}

		// 4) 注册默认 handler
		ka.RegisterDefaultHandlers(topics, msg.HandlerTopicMessage)
		ka.RegisterDefaultHandlers(ka.GenCAckTopic(), msg.HandlerCAckTopicMessage)

		// 5) 启动 ConsumerGroup
		_, err := ka.BootConsumers(allTopics)
		if err != nil {
			return
		}

		// 6) 阻塞直到 ctx.Done()
		select {
		case <-ctx.Done():
			glog.Infof("[Kafka] context done, shutting down")
		}
	}()
}

func ConfigMiddleware() {
	mid.Config()
}

func GetTenantID() string {
	return "tenant_001"
}

// 单个配置启动
//func BootKafka(appCfg *ka.AppConfig, handler ka.MessageHandler) {
//	go func() {
//		ctx, cancel := context.WithCancel(context.Background())
//		defer cancel()
//
//		// 1) 生成 topics
//		topics := ka.GenTopicsWithPattern(appCfg)
//		glog.Infof("[Kafka/%s] topics=%v", appCfg.GroupID, topics)
//
//		// 2) 创建 Topic（可选）
//		if appCfg.AutoCreateTopicsOnStart {
//			adminCfg := ka.BuildBaseConfigWith(appCfg)
//			admin, err := sarama.NewClusterAdmin(appCfg.Brokers, adminCfg)
//			if err != nil {
//				glog.Errorf("[Kafka/%s][ERR] create admin: %v", appCfg.GroupID, err)
//				return
//			}
//			if err := ka.EnsureTopicsWith(admin, topics, appCfg); err != nil {
//				glog.Errorf("[Kafka/%s][ERR] ensure topics: %v", appCfg.GroupID, err)
//				_ = admin.Close()
//				return
//			}
//			_ = admin.Close()
//		}
//
//		// 3) 初始化 client & producer
//		if err, _ := ka.InitKafkaClientWith(appCfg); err != nil {
//			glog.Errorf("[Kafka/%s][ERR] init client: %v", appCfg.GroupID, err)
//			return
//		}
//		if err := ka.InitSyncProducerFromClient(); err != nil {
//			glog.Errorf("[Kafka/%s][ERR] init producer: %v", appCfg.GroupID, err)
//			return
//		}
//
//		// 4) 注册 handler
//		if handler != nil {
//			ka.RegisterDefaultHandlers(topics, handler)
//		} else {
//			ka.RegisterDefaultHandlers(topics, msg.HandlerTopicMessage)
//			ka.RegisterDefaultHandlers(topics, msg.HandlerCAckTopicMessage)
//		}
//
//		// 5) 启动 consumer group
//		_, err := ka.BootConsumersWith(appCfg.GroupID, topics)
//		if err != nil {
//			glog.Errorf("[Kafka/%s][ERR] boot consumers: %v", appCfg.GroupID, err)
//			return
//		}
//
//		// 6) 阻塞，直到 ctx.Done()
//		<-ctx.Done()
//		glog.Infof("[Kafka/%s] shutting down", appCfg.GroupID)
//
//		ka.CloseSyncProducer()
//		ka.CloseKafkaClient()
//	}()
//}
