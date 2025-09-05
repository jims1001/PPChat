package msgflow

import (
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
	c := d.DB.Collection(collSeqConv)
	now := time.Now()

	filter := bson.M{"tenant_id": tenantID, "conversation_id": conversationID}
	update := bson.M{
		"$inc":         bson.M{"issued_seq": block},
		"$setOnInsert": bson.M{"max_seq": int64(0), "min_seq": int64(0), "create_time": now},
		"$set":         bson.M{"update_time": now},
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
	c := d.DB.Collection(collSeqConv)
	_, err := c.UpdateOne(ctx,
		bson.M{"tenant_id": tenantID, "conversation_id": conversationID},
		bson.M{"$max": bson.M{"max_seq": toSeq}, "$set": bson.M{"update_time": time.Now()}},
		options.Update().SetUpsert(true),
	)
	return err
}

// AdvanceMin 历史清理后推进 min_seq（保护：newMin <= max_seq）
func (d *DAO) AdvanceMin(ctx context.Context, tenantID, conversationID string, newMin int64) error {
	c := d.DB.Collection(collSeqConv)
	cond := bson.M{"tenant_id": tenantID, "conversation_id": conversationID, "max_seq": bson.M{"$gte": newMin}}
	_, err := c.UpdateOne(ctx, cond,
		bson.M{"$max": bson.M{"min_seq": newMin}, "$set": bson.M{"update_time": time.Now()}},
	)
	return err
}

// RaiseIssuedFloor 纠偏：抬高 issued_seq 下限（Redis 回退/冷启动时）
func (d *DAO) RaiseIssuedFloor(ctx context.Context, tenantID, conversationID string, floor int64) error {
	c := d.DB.Collection(collSeqConv)
	_, err := c.UpdateOne(ctx,
		bson.M{"tenant_id": tenantID, "conversation_id": conversationID},
		bson.M{"$max": bson.M{"issued_seq": floor}, "$set": bson.M{"update_time": time.Now()}},
		options.Update().SetUpsert(true),
	)
	return err
}
