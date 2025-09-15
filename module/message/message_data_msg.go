package message

import (
	pb "PProject/gen/message"
	"PProject/logger"
	seq2 "PProject/module/chat/seq"
	"PProject/service/chat"
	"PProject/service/mgo"
	"PProject/service/storage/redis"
	"context"
	"fmt"

	msgcli "PProject/service/msg"
	util "PProject/tools"

	chatModel "PProject/module/chat/model"
	chatService "PProject/module/chat/service"
)

func HandlerTopicMessage(topic string, key, value []byte) error {

	msg, err := util.DecodeFrame(value)
	if err != nil {
		logger.Errorf("topic key :%v Parse msg error: %s", topic, err)
		return err
	}

	ctx := context.Context(context.Background())
	/// 写数据库
	if msg.Type == pb.MessageFrameData_DATA {

		// 创建索引
		_ = seq2.EnsureIndexes(ctx)
		// 获取到回话ID
		convId, _, _ := seq2.EnsureSeqConversation(ctx, "tenant_001", msg.From, msg.To, int32(seq2.ConvTypeP2P))

		logger.Infof("topic key:%v convId:%v", topic, convId)

		dao := &seq2.DAO{DB: mgo.GetDB()}

		// 分配seq
		alloc := &seq2.Allocator{
			Rdb: redis.GetRedis(),
			DAO: dao,
			// 下面两个可选，不设就用默认：
			BlockSizeFn: nil, // 自适应段大小，nil 用默认策略
			KeyFn:       nil, // Redis key 生成规则，nil 用 "seq:blk:tenant:conv"
			MaxRetry:    5,   // 缺段/冲突时重试次数，默认10
		}

		// 获取到seq
		start, mill, err := alloc.Malloc(ctx, "tenant_001", convId, 1)
		_, _, err = seq2.EnsureTwoSidesByKnownConvID(ctx, "tenant_001", convId, int32(seq2.ConvTypeP2P), msg.From, msg.To, start)
		if err != nil {
			logger.Errorf("topic key:%v Parse msg error: %s", topic, err)
		}

		logger.Infof("topic key:%v start:%v mill:%v", topic, start, mill)

		// 根据seq 插入消息
		newMsg, err := chatService.BuildMessageModelFromPB("tenant_001", msg.GetPayload(), start, convId)
		if err != nil {
			logger.Errorf("topic key:%v build msg error: %s", topic, err)
			return err
		}

		// 插入消息
		err = chatModel.InsertMessage(ctx, newMsg)
		if err != nil {
			logger.Errorf("topic key:%v InsertMessage  error: %s", topic, err)
			return err
		}

		// 设置最大的seq
		seq, err := seq2.UpdateMaxSeq(ctx, convId, start)
		if err != nil {
			return err
		}

		if seq != start {
			logger.Errorf("topic key:%v UpdateMaxSeq  error: %s", topic, err)
			return fmt.Errorf("topic key:%v seq diff error", topic)
		}

		//msg.WsRelayBound <- msg
		err = msgcli.ReplayMsg(value)
		if err != nil {
			logger.Infof("topic key :%v Replay msg error: %s", topic, err)
			return err
		}

		deliverMsg := chat.BuildSendSuccessAckDeliver(msg.From, msg.GetPayload().ClientMsgId, msg.GetPayload().ServerMsgId, msg)
		deliverMsgData, err := util.EncodeFrame(deliverMsg)
		if err != nil {
			logger.Errorf("BuildSendSuccessAckDeliver EncodeFrame topic key :%v Replay msg error: %s", topic, err)
			return err
		}
		err = msgcli.ReplayMsg(deliverMsgData)
		if err != nil {
			logger.Errorf("BuildSendSuccessAckDeliverv ReplayMsg topic key :%v Replay msg error: %s", topic, err)
			return err
		}
	}

	// 下发给自己
	return nil
}
