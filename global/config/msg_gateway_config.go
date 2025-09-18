package config

import pb "PProject/gen/message"

var MessageGatewayConfig = AppConfig{
	NodeType: NodeTypeMsgGateWay,        // 网关节点
	GroupId:  "message_gateway_node_01", // kafka group 节点
	NodeId:   "gateway_01",              // 节点ID
	ReceiveTopic: map[pb.MessageFrameData_Type]string{
		pb.MessageFrameData_DATA: "message_receive_data",
		pb.MessageFrameData_CACK: "message_receive_ack",
	}, // 接收消息的topic
	SendMsgTopic: map[pb.MessageFrameData_Type]string{
		pb.MessageFrameData_DATA: "message_sender_data",
		pb.MessageFrameData_CACK: "message_sender_ack",
	},
	Port:     8080,
	GrpcPort: 50051,
}

var MessageGatewayConfigV1 = AppConfig{
	NodeType: NodeTypeMsgGateWay,        // 网关节点
	GroupId:  "message_gateway_node_01", // kafka group 节点
	NodeId:   "gateway_02",              // 节点ID
	ReceiveTopic: map[pb.MessageFrameData_Type]string{
		pb.MessageFrameData_DATA: "message_receive_data",
		pb.MessageFrameData_CACK: "message_receive_ack",
	}, // 接收消息的topic
	SendMsgTopic: map[pb.MessageFrameData_Type]string{
		pb.MessageFrameData_DATA: "message_sender_data",
		pb.MessageFrameData_CACK: "message_sender_ack",
	},
	Port:     9090,
	GrpcPort: 50051,
}

var MessageDataConfig = AppConfig{
	NodeType: NodeTypeDataNode,       // 消息节点
	GroupId:  "message_data_node_02", // kafka group 节点
	NodeId:   "date_01",              // 节点ID
	ReceiveTopic: map[pb.MessageFrameData_Type]string{
		pb.MessageFrameData_DATA: "message_receive_data",
		pb.MessageFrameData_CACK: "message_receive_ack",
	}, // 接收消息的topic
	SendMsgTopic: map[pb.MessageFrameData_Type]string{
		pb.MessageFrameData_DATA: "message_sender_data",
		pb.MessageFrameData_CACK: "message_sender_ack",
	},
	Port:     8083,
	GrpcPort: 50052,
}

var MessageApiNodeConfig = AppConfig{
	NodeType: NodeTypeApiNode,
	NodeId:   "apiNode_01",
	GroupId:  "message_api_node_03",
	ReceiveTopic: map[pb.MessageFrameData_Type]string{
		pb.MessageFrameData_DATA: "message_receive_data",
		pb.MessageFrameData_CACK: "message_receive_ack",
	}, // 接收消息的topic
	SendMsgTopic: map[pb.MessageFrameData_Type]string{
		pb.MessageFrameData_DATA: "message_sender_data",
		pb.MessageFrameData_CACK: "message_sender_ack",
	},
	Port:     9090,
	GrpcPort: 50053,
}
