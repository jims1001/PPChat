package store

import (
	"context"
	"time"

	"PProject/module/chatBox/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repo struct {
	DB *mongo.Database
}

func ptr[T any](v T) *T { return &v }

// AppendEvent AppendEvent: 原子自增 seq → upsert 事件
func (r *Repo) AppendEvent(ctx context.Context, ev *model.ConversationEvent) (int64, error) {
	sess, err := r.DB.Client().StartSession()
	if err != nil {
		return 0, err
	}
	defer sess.EndSession(ctx)

	var outSeq int64
	_, err = sess.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
		// 1) seq ++
		after := options.After
		res := r.DB.Collection("conv_seq").FindOneAndUpdate(sc,
			bson.M{"tenant_id": ev.TenantID, "conversation_id": ev.ConversationID},
			bson.M{"$inc": bson.M{"max_seq": 1}},
			&options.FindOneAndUpdateOptions{Upsert: ptr(true), ReturnDocument: &after},
		)
		var doc struct {
			MaxSeq int64 `bson:"max_seq"`
		}
		if err := res.Decode(&doc); err != nil {
			return nil, err
		}
		ev.Seq = doc.MaxSeq
		if ev.CreatedAtMS == 0 {
			ev.CreatedAtMS = time.Now().UnixMilli()
		}
		outSeq = ev.Seq

		// 2) upsert event（幂等）
		_, err := r.DB.Collection("conversation_events").UpdateOne(sc,
			bson.M{"event_id": ev.EventID},
			bson.M{"$setOnInsert": ev},
			options.Update().SetUpsert(true),
		)
		return nil, err
	})
	return outSeq, err
}
