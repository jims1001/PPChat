package seq

import (
	chatmodel "PProject/module/chat/model"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const collSeqConv = "seq_conversation"

type DAO struct{ DB *mongo.Database }

// AllocSegment 原子从 Mongo 领一段：issued_seq += block，返回 [start,end]
func (d *DAO) AllocSegment(ctx context.Context, tenantID, conversationID string, block int64) (start, end int64, err error) {
	if block <= 0 {
		block = 256
	}
	seq := chatmodel.SeqConversation{}
	c := d.DB.Collection(seq.GetTableName())
	now := time.Now()

	filter := bson.M{chatmodel.SeqConvFieldTenantID: tenantID,
		chatmodel.SeqConvFieldConversationID: conversationID}
	update := bson.M{
		"$inc":         bson.M{chatmodel.SeqConvFieldIssuedSeq: block},
		"$setOnInsert": bson.M{chatmodel.SeqConvFieldMaxSeq: int64(0), chatmodel.SeqConvFieldMinSeq: int64(0), chatmodel.SeqConvFieldCreateTime: now},
		"$set":         bson.M{chatmodel.SeqConvFieldUpdateTime: now},
	}

	var before struct {
		IssuedSeq int64 `bson:"issued_seq"`
	}
	err = c.FindOneAndUpdate(
		ctx, filter, update,
		options.FindOneAndUpdate().
			SetUpsert(true).
			SetReturnDocument(options.Before),
	).Decode(&before)
	if err != nil && err != mongo.ErrNoDocuments {
		return 0, 0, err
	}
	old := before.IssuedSeq // 不存在时视为0
	return old + 1, old + block, nil
}

// AdvanceCommit 提交可读水位：max_seq = max(max_seq, toSeq)
func (d *DAO) AdvanceCommit(ctx context.Context, tenantID, conversationID string, toSeq int64) error {
	seq := chatmodel.SeqConversation{}
	c := d.DB.Collection(seq.GetTableName())

	_, err := c.UpdateOne(ctx,
		bson.M{chatmodel.SeqConvFieldTenantID: tenantID, chatmodel.SeqConvFieldConversationID: conversationID},
		bson.M{"$max": bson.M{chatmodel.SeqConvFieldMaxSeq: toSeq}, "$set": bson.M{chatmodel.SeqConvFieldUpdateTime: time.Now()}},
		options.Update().SetUpsert(true),
	)
	return err
}

// AdvanceMin 历史清理后推进 min_seq（保护：newMin <= max_seq）
func (d *DAO) AdvanceMin(ctx context.Context, tenantID, conversationID string, newMin int64) error {
	seq := chatmodel.SeqConversation{}
	c := d.DB.Collection(seq.GetTableName())

	cond := bson.M{chatmodel.SeqConvFieldTenantID: tenantID,
		chatmodel.SeqConvFieldConversationID: conversationID,
		chatmodel.SeqConvFieldMaxSeq:         bson.M{"$gte": newMin}}
	_, err := c.UpdateOne(ctx, cond,
		bson.M{"$max": bson.M{chatmodel.SeqConvFieldMinSeq: newMin},
			"$set": bson.M{chatmodel.SeqConvFieldUpdateTime: time.Now()}},
	)
	return err
}

// RaiseIssuedFloor 纠偏：抬高 issued_seq 下限（Redis 回退/冷启动时）
func (d *DAO) RaiseIssuedFloor(ctx context.Context, tenantID, conversationID string, floor int64) error {
	seq := chatmodel.SeqConversation{}
	c := d.DB.Collection(seq.GetTableName())

	_, err := c.UpdateOne(ctx,
		bson.M{chatmodel.SeqConvFieldTenantID: tenantID,
			chatmodel.SeqConvFieldConversationID: conversationID},
		bson.M{"$max": bson.M{chatmodel.SeqConvFieldIssuedSeq: floor},
			"$set": bson.M{chatmodel.SeqConvFieldUpdateTime: time.Now()}},
		options.Update().SetUpsert(true),
	)
	return err
}
