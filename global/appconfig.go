package global

type AppConfig struct {
	NodeType      string
	GroupId       string
	SendMsgTopic  string // 设置发送消息的topic
	ReceiveTopic  string // 接收消息的topic
	GatewayNodeId string // 节点的Id
	Port          int    // http 启动端口
	GrpcPort      int
}
