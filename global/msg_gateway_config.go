package global

var MessageGatewayConfig = AppConfig{
	NodeType:      NodeTypeMsgGateWay,   // 网关节点
	GroupId:       "message_gateway_01", // kafka group 节点
	GatewayNodeId: "gateway_01",         // 节点ID
	ReceiveTopic:  "msg_receive_topic",  // 接收消息的topic
	SendMsgTopic:  "msg_send_topic",     // 发送消息的topic
	Port:          8080,
	GrpcPort:      50051,
}

var MessageDataConfig = AppConfig{
	NodeType:      NodeTypeDataNode,    // 消息节点
	GroupId:       "message_node_01",   // kafka group 节点
	GatewayNodeId: "date_01",           // 节点ID
	ReceiveTopic:  "msg_receive_topic", // 接收消息的topic
	SendMsgTopic:  "msg_send_topic",    // 发送消息的topic
	Port:          8083,
	GrpcPort:      50052,
}
