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
		glog.Infof("[Kafka] topics=%v", topics)

		// 2) 启动前（可选）创建 Topic
		if ka.Cfg.AutoCreateTopicsOnStart {
			adminCfg := ka.BuildBaseConfig()
			admin, err := sarama.NewClusterAdmin(ka.Cfg.Brokers, adminCfg)
			if err != nil {
				glog.Infof("[Kafka][ERR] create admin: %v", err)
				return
			}
			if err := ka.EnsureTopics(admin, topics); err != nil {
				glog.Infof("[Kafka][ERR] ensure topics: %v", err)
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

		// 5) 启动 ConsumerGroup
		_, err := ka.BootConsumers(topics)
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
