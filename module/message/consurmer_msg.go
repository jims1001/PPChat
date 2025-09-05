package message

import (
	pb "PProject/gen/message"
	"PProject/service/chat"
	"PProject/service/mgo"
	"PProject/service/storage/redis"
	"context"

	msgcli "PProject/service/msg"
	util "PProject/tools"

	flow "PProject/module/message/msgflow"

	"github.com/golang/glog"
)

func HandlerTopicMessage(topic string, key, value []byte) error {
	msg, err := util.DecodeFrame(value)
	if err != nil {
		glog.Infof("topic key :%v Parse msg error: %s", topic, err)
		return err
	}

	ctx := context.Context(context.Background())
	/// 写数据库
	if msg.Type == pb.MessageFrameData_DATA {

		convId, _, _ := flow.EnsureP2PForOwner(ctx, "tenant_001", msg.From, msg.To, int32(flow.ConvTypeP2P))
		glog.Infof("topic key:%v convId:%v", topic, convId)

		dao := &flow.DAO{DB: mgo.GetDB()}

		alloc := &flow.Allocator{
			Rdb: redis.GetRedis(),
			DAO: dao,
			// 下面两个可选，不设就用默认：
			BlockSizeFn: nil, // 自适应段大小，nil 用默认策略
			KeyFn:       nil, // Redis key 生成规则，nil 用 "seq:blk:tenant:conv"
			MaxRetry:    5,   // 缺段/冲突时重试次数，默认10
		}

		start, mill, err := alloc.Malloc(ctx, "tenant_001", convId, 1)
		glog.Infof("topic key:%v start:%v mill:%v", topic, start, mill)

		if err != nil {
			glog.Infof("topic key:%v Parse msg error: %s", topic, err)
			return err
		}

		//msg.WsRelayBound <- msg
		err = msgcli.ReplayMsg(value)
		if err != nil {
			glog.Infof("topic key :%v Replay msg error: %s", topic, err)
			return err
		}

		deliverMsg := chat.BuildSendSuccessAckDeliver(msg.From, msg.GetPayload().ClientMsgId, msg.GetPayload().ServerMsgId, msg)
		deliverMsgData, err := util.EncodeFrame(deliverMsg)
		if err != nil {
			glog.Infof("BuildSendSuccessAckDeliver EncodeFrame topic key :%v Replay msg error: %s", topic, err)
			return err
		}
		err = msgcli.ReplayMsg(deliverMsgData)
		if err != nil {
			glog.Infof("BuildSendSuccessAckDeliverv ReplayMsg topic key :%v Replay msg error: %s", topic, err)
			return err
		}
	}

	// 下发给自己
	//glog.Infof("[TestTopic] key=%s, value=%s", key, value)
	return nil
}
