package config

import (
	"PProject/data/database/mgo/mongoutil"
	pb "PProject/gen/message"
	"PProject/logger"
	mid "PProject/middleware"
	"PProject/service/dispatcher/kafka"
	mgoSrv "PProject/service/mgo"
	redis "PProject/service/storage/redis"
	ids "PProject/tools/ids"
	"context"

	"github.com/Shopify/sarama"
)

const NodeTypeMsgGateWay = "msgGateWay" // 网关节点
const NodeTypeDataNode = "msgDataNode"  // 数据节点
const NodeTypeApiNode = "apiNode"

var Global = AppConfig{
	NodeType: NodeTypeMsgGateWay,  // 是否是消息网关
	GroupId:  "im-app-consumer-1", // kafka group 节点
	NodeId:   "gateway_10",        // 节点ID
	ReceiveTopic: map[pb.MessageFrameData_Type]string{
		pb.MessageFrameData_DATA: "message_sender_data",
		pb.MessageFrameData_CACK: "message_sender_ack",
	}, // 接收消息的topic
	SendMsgTopic: map[pb.MessageFrameData_Type]string{
		pb.MessageFrameData_DATA: "message_receive_data",
		pb.MessageFrameData_CACK: "message_receive_ack",
	}, // 发送消息的topic
	Port:     8080,
	GrpcPort: 50051,
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

	isSenderConsumer := false
	nodeId := MessageGatewayConfig.NodeId
	if Global.NodeType == NodeTypeDataNode {
		isSenderConsumer = true
		nodeId = MessageGatewayConfig.NodeId
	} else if Global.NodeType == NodeTypeApiNode {
		isSenderConsumer = true
	}

	// 网关的节点 就不需要接受 发送的topic 只需要管 接收的topic就可以了
	dataCfg := kafka.MessageHandlerConfig{
		Handler:             handler,
		SendTopicPattern:    Global.SendMsgTopic[pb.MessageFrameData_DATA],
		ReceiveTopicPattern: Global.ReceiveTopic[pb.MessageFrameData_DATA],
		MessageType:         pb.MessageFrameData_DATA,
		TopicCount:          2,
		IsConsumer:          isSenderConsumer,
		NodeId:              nodeId,
	}

	cackCfg := kafka.MessageHandlerConfig{
		Handler:             handler,
		SendTopicPattern:    Global.SendMsgTopic[pb.MessageFrameData_CACK],
		ReceiveTopicPattern: Global.ReceiveTopic[pb.MessageFrameData_CACK],
		MessageType:         pb.MessageFrameData_CACK,
		IsConsumer:          isSenderConsumer,
		TopicCount:          2,
		NodeId:              nodeId,
	}

	kafka.Cfg.GroupID = Global.GroupId
	kafka.Cfg.MessageConfigs = []kafka.MessageHandlerConfig{dataCfg, cackCfg}

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 2) 启动前（可选）创建 Topic
		if kafka.Cfg.AutoCreateTopicsOnStart {
			adminCfg := kafka.BuildBaseConfig()
			admin, err := sarama.NewClusterAdmin(kafka.Cfg.Brokers, adminCfg)
			if err != nil {
				logger.Infof("[Kafka][ERR] create admin: %v", err)
				return
			}
			if err := kafka.EnsureTopics(ctx, kafka.Cfg); err != nil {
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

		keys := kafka.Cfg.GetAllTopicKeys()
		// 5) 启动 ConsumerGroup
		_, err := kafka.BootConsumers(keys)
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
