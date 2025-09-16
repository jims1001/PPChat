package config

import (
	pb "PProject/gen/message"
	ka "PProject/service/dispatcher/kafka"
)

type AppConfig struct {
	NodeType     string
	GroupId      string
	SendMsgTopic map[pb.MessageFrameData_Type]string // 设置发送消息的topic
	ReceiveTopic map[pb.MessageFrameData_Type]string // 接收消息的topic
	NodeId       string                              // 节点的Id
	Port         int                                 // http 启动端口
	GrpcPort     int
	TopicHandler ka.MessageHandler // 消息处理handler
}
