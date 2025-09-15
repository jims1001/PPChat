package global

import (
	"PProject/data/database/mgo/mongoutil"
	"PProject/logger"
	mid "PProject/middleware"
	msg "PProject/module/message"
	"PProject/service/dispatcher/kafka"
	mgoSrv "PProject/service/mgo"
	redis "PProject/service/storage/redis"
	ids "PProject/tools/ids"
	"context"

	"github.com/Shopify/sarama"
)

const NodeTypeMsgGateWay = "msgGateWay" // 网关节点
const NodeTypeDataNode = "msgDataNode"  // 数据节点

var GlobalConfig = AppConfig{
	NodeType:      NodeTypeMsgGateWay,  // 是否是消息网关
	GroupId:       "im-app-consumer-1", // kafka group 节点
	GatewayNodeId: "gateway_10",        // 节点ID
	ReceiveTopic:  "msg_receive_topic", // 接收消息的topic
	SendMsgTopic:  "msg_send_topic",    // 发送消息的topic
}

type AppConfig struct {
	NodeType      string
	GroupId       string
	SendMsgTopic  string // 设置发送消息的topic
	ReceiveTopic  string // 接收消息的topic
	GatewayNodeId string // 节点的Id
}

func ConfigAll() {
	ConfigIds()
	ConfigRedis()
	ConfigMgo()
	//ConfigKafka()

}

func ConfigIds() {

	logger.Infof("配置id生成")
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
func ConfigKafka(handler kafka.MessageHandler) {
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 1) 生成所有大 Topic 名称
		topics := kafka.GenTopics()
		allTopics := append(kafka.GenTopics(), kafka.GenCAckTopic()...)
		logger.Infof("[Kafka] allTopics=%v", allTopics)

		// 2) 启动前（可选）创建 Topic
		if kafka.Cfg.AutoCreateTopicsOnStart {
			adminCfg := kafka.BuildBaseConfig()
			admin, err := sarama.NewClusterAdmin(kafka.Cfg.Brokers, adminCfg)
			if err != nil {
				logger.Infof("[Kafka][ERR] create admin: %v", err)
				return
			}
			if err := kafka.EnsureTopics(admin, allTopics); err != nil {
				logger.Infof("[Kafka][ERR] ensure allTopics: %v", err)
				_ = admin.Close()
				return
			}
			_ = admin.Close()
		}

		// 3) 初始化 Client & Producer
		if err := kafka.InitKafkaClient(); err != nil {
			logger.Infof("[Kafka][ERR] init client: %v", err)
			return
		}
		if err := kafka.InitSyncProducerFromClient(); err != nil {
			logger.Infof("[Kafka][ERR] init producer: %v", err)
			return
		}

		// 4) 注册默认 handler
		kafka.RegisterDefaultHandlers(topics, msg.HandlerTopicMessage)
		kafka.RegisterDefaultHandlers(kafka.GenCAckTopic(), msg.HandlerCAckTopicMessage)

		// 5) 启动 ConsumerGroup
		_, err := kafka.BootConsumers(allTopics)
		if err != nil {
			return
		}

		// 6) 阻塞直到 ctx.Done()
		select {
		case <-ctx.Done():
			logger.Infof("[Kafka] context done, shutting down")
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
//		logger.Infof("[Kafka/%s] topics=%v", appCfg.GroupID, topics)
//
//		// 2) 创建 Topic（可选）
//		if appCfg.AutoCreateTopicsOnStart {
//			adminCfg := ka.BuildBaseConfigWith(appCfg)
//			admin, err := sarama.NewClusterAdmin(appCfg.Brokers, adminCfg)
//			if err != nil {
//				logger.Errorf("[Kafka/%s][ERR] create admin: %v", appCfg.GroupID, err)
//				return
//			}
//			if err := ka.EnsureTopicsWith(admin, topics, appCfg); err != nil {
//				logger.Errorf("[Kafka/%s][ERR] ensure topics: %v", appCfg.GroupID, err)
//				_ = admin.Close()
//				return
//			}
//			_ = admin.Close()
//		}
//
//		// 3) 初始化 client & producer
//		if err, _ := ka.InitKafkaClientWith(appCfg); err != nil {
//			logger.Errorf("[Kafka/%s][ERR] init client: %v", appCfg.GroupID, err)
//			return
//		}
//		if err := ka.InitSyncProducerFromClient(); err != nil {
//			logger.Errorf("[Kafka/%s][ERR] init producer: %v", appCfg.GroupID, err)
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
//			logger.Errorf("[Kafka/%s][ERR] boot consumers: %v", appCfg.GroupID, err)
//			return
//		}
//
//		// 6) 阻塞，直到 ctx.Done()
//		<-ctx.Done()
//		logger.Infof("[Kafka/%s] shutting down", appCfg.GroupID)
//
//		ka.CloseSyncProducer()
//		ka.CloseKafkaClient()
//	}()
//}
