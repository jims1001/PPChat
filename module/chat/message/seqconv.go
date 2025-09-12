package message

import (
	"context"
	"time"

	chatmodel "PProject/module/chat/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *Store) EnsureSeqConversation(ctx context.Context, tenant, conv string) error {
	_, err := s.SeqConvColl.UpdateOne(ctx,
		bson.M{chatmodel.SeqConvFieldTenantID: tenant,
			chatmodel.SeqConvFieldConversationID: conv},
		bson.M{"$setOnInsert": bson.M{
			chatmodel.SeqConvFieldMinSeq:     int64(0),
			chatmodel.SeqConvFieldMaxSeq:     int64(0),
			chatmodel.SeqConvFieldUpdateTime: time.Now().UnixMilli(),
		}},
		options.Update().SetUpsert(true),
	)
	return err
}

func (s *Store) BumpMaxSeq(ctx context.Context, tenant, conv string, seq int64) error {
	_, err := s.SeqConvColl.UpdateOne(ctx,
		bson.M{chatmodel.SeqConvFieldTenantID: tenant,
			chatmodel.SeqConvFieldConversationID: conv},
		bson.M{"$max": bson.M{chatmodel.SeqConvFieldMaxSeq: seq},
			"$set": bson.M{chatmodel.SeqConvFieldUpdateTime: time.Now().UnixMilli()}},
	)
	return err
}

func (s *Store) BumpMinSeq(ctx context.Context, tenant, conv string, seq int64) error {
	_, err := s.SeqConvColl.UpdateOne(ctx,
		bson.M{chatmodel.SeqConvFieldTenantID: tenant,
			chatmodel.SeqConvFieldConversationID: conv},
		bson.M{"$max": bson.M{chatmodel.SeqConvFieldMinSeq: seq},
			"$set": bson.M{chatmodel.SeqConvFieldUpdateTime: time.Now().UnixMilli()}},
	)
	return err
}
