package message

import (
	chatmodel "PProject/module/chat/model"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AddMentions 写消息时，解析到的被@/回复用户，插入倒排
func (s *Store) AddMentions(ctx context.Context, tenant, conv string, targetUserIDs []string, seq int64, kind string) error {
	if len(targetUserIDs) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	var writes []interface{}
	for _, t := range targetUserIDs {
		writes = append(writes, chatmodel.MentionIndex{
			TenantID: tenant, ConversationID: conv, TargetUserID: t, Seq: seq, Kind: kind, CreatedAt: now,
		})
	}
	if len(writes) > 0 {
		_, err := s.MentionColl.InsertMany(ctx, writes, options.InsertMany().SetOrdered(false))
		if err != nil {
			return err
		}
	}
	// 角标快路径（可选）：对应用户的 conversation.MentionUnread += len(mentions to him)
	for _, t := range targetUserIDs {
		_, _ = s.ConvColl.UpdateOne(ctx,
			bson.M{chatmodel.ConversationFieldTenantID: tenant,
				chatmodel.ConversationFieldOwnerUserID:    t,
				chatmodel.ConversationFieldConversationID: conv},
			bson.M{"$inc": bson.M{chatmodel.ConversationFieldMentionUnread: 1},
				"$set": bson.M{chatmodel.ConversationFieldUpdatedAt: time.Now().UnixMilli()}},
			options.Update().SetUpsert(true),
		)
	}
	return nil
}

// DeductMentionsByRange 连续已读到 S：扣 (mention_read_seq, S] 的 @ 数
func (s *Store) DeductMentionsByRange(ctx context.Context, tenant, owner, conv string, oldMentionRead, S int64) error {
	if S <= oldMentionRead {
		return nil
	}
	filter := bson.M{
		chatmodel.MIFieldTenantID:       tenant,
		chatmodel.MIFieldConversationID: conv,
		chatmodel.MIFieldTargetUserID:   owner,
		chatmodel.MIFieldSeq:            bson.M{"$gt": oldMentionRead, "$lte": S},
	}
	cnt, err := s.MentionColl.CountDocuments(ctx, filter)
	if err != nil {
		return err
	}
	if cnt > 0 {
		_, err = s.ConvColl.UpdateOne(ctx,
			bson.M{chatmodel.ConversationFieldTenantID: tenant,
				chatmodel.ConversationFieldOwnerUserID:    owner,
				chatmodel.ConversationFieldConversationID: conv},
			bson.M{"$inc": bson.M{chatmodel.ConversationFieldMentionUnread: -cnt},
				"$max": bson.M{chatmodel.ConversationFieldMentionReadSeq: S},
				"$set": bson.M{chatmodel.ConversationFieldUpdatedAt: time.Now().UnixMilli()}},
		)
		return err
	}
	_, err = s.ConvColl.UpdateOne(ctx,
		bson.M{chatmodel.ConversationFieldTenantID: tenant,
			chatmodel.ConversationFieldOwnerUserID:    owner,
			chatmodel.ConversationFieldConversationID: conv},
		bson.M{"$max": bson.M{chatmodel.ConversationFieldMentionReadSeq: S},
			"$set": bson.M{chatmodel.ConversationFieldUpdatedAt: time.Now().UnixMilli()}},
	)
	return err
}

// DeductMentionsBySeqs 跳读新增点：按本次“新增置位”的 seq 列表扣 @
func (s *Store) DeductMentionsBySeqs(ctx context.Context, tenant, owner, conv string, seqs []int64) error {
	if len(seqs) == 0 {
		return nil
	}
	filter := bson.M{
		chatmodel.MIFieldTenantID:       tenant,
		chatmodel.MIFieldConversationID: conv,
		chatmodel.MIFieldTargetUserID:   owner,
		chatmodel.MIFieldSeq:            bson.M{"$in": seqs},
	}
	cnt, err := s.MentionColl.CountDocuments(ctx, filter)
	if err != nil {
		return err
	}
	if cnt > 0 {
		_, err = s.ConvColl.UpdateOne(ctx,
			bson.M{chatmodel.ConversationFieldTenantID: tenant,
				chatmodel.ConversationFieldOwnerUserID:    owner,
				chatmodel.ConversationFieldConversationID: conv},
			bson.M{"$inc": bson.M{chatmodel.ConversationFieldMentionUnread: -cnt},
				"$set": bson.M{chatmodel.ConversationFieldUpdatedAt: time.Now().UnixMilli()}},
		)
		return err
	}
	return nil
}

// FirstUnreadMentionSeq 定位第一个未读@（排除已读前缀和跳读）
func (s *Store) FirstUnreadMentionSeq(ctx context.Context, tenant, owner, conv string, readSeq int64) (int64, error) {
	// 这里简化：只看 > readSeq；若要排除“跳读已置位”，可额外交叉 SparseBlocks（略）
	cur, err := s.MentionColl.Find(ctx, bson.M{
		chatmodel.MIFieldTenantID:       tenant,
		chatmodel.MIFieldConversationID: conv,
		chatmodel.MIFieldTargetUserID:   owner,
		chatmodel.MIFieldSeq:            bson.M{"$gt": readSeq},
	}, options.Find().SetSort(bson.M{"seq": 1}).SetLimit(1))
	if err != nil {
		return 0, err
	}
	defer func(cur *mongo.Cursor, ctx context.Context) {
		err := cur.Close(ctx)
		if err != nil {

		}
	}(cur, ctx)
	if cur.Next(ctx) {
		var m chatmodel.MentionIndex
		_ = cur.Decode(&m)
		return m.Seq, nil
	}
	return 0, nil
}
