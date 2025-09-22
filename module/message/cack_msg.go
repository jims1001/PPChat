package message

import (
	pb "PProject/gen/message"
	"PProject/logger"
	chatmodel "PProject/module/chat/model"
	"PProject/module/chat/service"
	msgcli "PProject/service/msg"
	util "PProject/tools"
	"context"
)

func HandlerCAckTopicMessage(topic string, key, value []byte) error {
	ctx := context.Context(context.Background())
	defer ctx.Done()
	msg, err := util.DecodeFrame(value)
	if err != nil {
		logger.Errorf("topic key :%v Parse msg error: %s", topic, err)
		return err
	}

	if msg.Type == pb.MessageFrameData_CACK {
		msgId := msg.GetPayload().ServerMsgId
		if msgId != "" {

			message, _ := chatmodel.GetMessageByServerMsgID(ctx, msgId)
			if message != nil {
				seq := msg.GetPayload().Seq
				conversation := chatmodel.Conversation{}
				minSeq, err := conversation.UpdateMinSeq(ctx, "tenant_001", message.ConversationID, seq)
				if err != nil {
					return err
				}
				logger.Infof("topic key :%v Update min seq:%v", topic, minSeq)

				c := chatmodel.Conversation{}
				conv, err := c.GetConversationByID(ctx, "tenant_001", message.ConversationID)
				if err != nil {
					return err
				}

				convList := []*chatmodel.Conversation{conv}
				replayMsg, err := service.BuildSyncFrameConversations(message.SendID, convList)
				if err != nil {
					return err
				}

				replayMsgData, err := util.EncodeFrame(replayMsg)

				err = msgcli.ReplayMsg(replayMsgData, msg.GetSessionId())
				if err != nil {
					logger.Errorf("topic key :%v Parse msg error: %s", topic, err)
					return err
				}

			}

		}

	}

	logger.Infof("topic key:%v Parse msg success", msg)

	return nil
}
