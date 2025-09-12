package seq

import (
	chatmodel "PProject/module/chat/model"
	"PProject/service/mgo"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/appengine/log"
)

const ConvTypeP2P int = 100
const ConvTypeGroup int = 200
const convTypeChannel int = 300

// EnsureSeqConversation 用于确保 “p2p:min_max” 这一条会话级水位存在；只维护水位和元数据
func EnsureSeqConversation(ctx context.Context, tenantID, userA, userB string, chatType int32) (conversationID string, created bool, err error) {

	conversationID = buildP2PConvID(userA, userB)

	var sc chatmodel.SeqConversation
	c := sc.Collection() // 这里还是你项目里的 collection 取法

	now := time.Now()
	filter := bson.M{
		chatmodel.SeqConvFieldTenantID:       tenantID,
		chatmodel.SeqConvFieldConversationID: conversationID,
	}
	update := bson.M{
		"$setOnInsert": bson.M{
			chatmodel.SeqConvFieldTenantID:       tenantID,
			chatmodel.SeqConvFieldConversationID: conversationID,
			chatmodel.SeqConvFieldMaxSeq:         int64(0),
			chatmodel.SeqConvFieldMinSeq:         int64(0),
			chatmodel.SeqConvFieldIssuedSeq:      int64(0),
			chatmodel.SeqConvFieldCompactWM:      int64(0),
			chatmodel.SeqConvFieldEpoch:          int32(0),
			chatmodel.SeqConvFieldShardKey:       "",
			chatmodel.SeqConvFieldCreateTime:     now,
		},
		"$set": bson.M{
			chatmodel.SeqConvFieldUpdateTime: now,
		},
	}

	res, err := c.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return "", false, err
	}
	return conversationID, res.UpsertedCount > 0, nil
}

func UpdateMaxSeq(ctx context.Context, conversationID string, newMax int64) (int64, error) {

	filter := bson.M{
		chatmodel.SeqConvFieldConversationID: conversationID,
	}
	update := bson.M{
		"$max": bson.M{chatmodel.SeqConvFieldMaxSeq: newMax},
		"$set": bson.M{chatmodel.SeqConvFieldUpdateTime: time.Now()},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).
		SetReturnDocument(options.After).
		SetProjection(bson.M{chatmodel.SeqConvFieldMaxSeq: 1})

	var out struct {
		MaxSeq int64 `bson:"max_seq"`
	}
	col := chatmodel.SeqConversation{}
	err := col.Collection().FindOneAndUpdate(ctx, filter, update, opts).Decode(&out)
	if err != nil {
		return 0, err
	}
	return out.MaxSeq, nil
}

// EnsureTwoSidesByKnownConvID
// 已知 covID：如果两条都存在 -> 只更新 server_max_seq；否则 -> 创建缺失的并刷新影子。
// 返回 createdA/createdB 表示 A/B 是否在本次被创建。
func EnsureTwoSidesByKnownConvID(ctx context.Context, tenantID string, conversationID string, // 已知 covId（p2p:min_max / grp:xxx）
	chatType int32, // 1=单聊 2=群聊 ...
	userA string, // A 作为 owner
	userB string, // B 作为 owner
	serverMaxSeq int64, // 本次影子水位（没有就传 0）
) (createdA bool, createdB bool, err error) {

	conv := chatmodel.Conversation{}
	// 1) 先看是否两条都在
	cnt, err := conv.Collection().CountDocuments(ctx, bson.M{
		chatmodel.ConversationFieldTenantID:       tenantID,
		chatmodel.ConversationFieldConversationID: conversationID,
		chatmodel.ConversationFieldOwnerUserID:    bson.M{"$in": []string{userA, userB}},
	})
	if err != nil {
		return false, false, err
	}

	now := time.Now()

	// 2) 两条都存在：只更新 server_max_seq（用 $max 防止回退）
	if cnt == 2 {
		_, err = conv.Collection().UpdateMany(ctx,
			bson.M{
				chatmodel.ConversationFieldTenantID:       tenantID,
				chatmodel.ConversationFieldConversationID: conversationID,
				chatmodel.ConversationFieldOwnerUserID:    bson.M{"$in": []string{userA, userB}},
			},
			bson.M{
				"$max": bson.M{chatmodel.ConversationFieldServerMaxSeq: serverMaxSeq},
				"$set": bson.M{chatmodel.ConversationFieldUpdatedAt: now},
			},
		)
		return false, false, err
	}

	// 3) 少于两条：分别 upsert A、B（存在即更新影子，不存在则创建）
	// 注意：插入时只写必要字段；其余个性化字段按你的默认即可
	up := func(owner, peer string) (bool, error) {
		now := time.Now()

		// Step 1: 仅负责插入（server_max_seq 初始化为 0），不做 $max，避免冲突
		res, err := conv.Collection().UpdateOne(ctx,
			bson.M{
				chatmodel.ConversationFieldTenantID:       tenantID,
				chatmodel.ConversationFieldOwnerUserID:    owner,
				chatmodel.ConversationFieldConversationID: conversationID,
			},
			bson.M{
				"$setOnInsert": bson.M{
					chatmodel.ConversationFieldTenantID:              tenantID,
					chatmodel.ConversationFieldOwnerUserID:           owner,
					chatmodel.ConversationFieldConversationID:        conversationID,
					chatmodel.ConversationFieldConversationType:      chatType,
					chatmodel.ConversationFieldUserID:                peer, // 单聊对端；群聊可留空
					chatmodel.ConversationFieldGroupID:               "",
					chatmodel.ConversationFieldRecvMsgOpt:            int32(0),
					chatmodel.ConversationFieldIsPinned:              false,
					chatmodel.ConversationFieldIsPrivateChat:         false,
					chatmodel.ConversationFieldBurnDuration:          int32(0),
					chatmodel.ConversationFieldGroupAtType:           int32(0),
					chatmodel.ConversationFieldAttachedInfo:          "",
					chatmodel.ConversationFieldEx:                    "",
					chatmodel.ConversationFieldReadSeq:               int64(0), // 个人读游标
					chatmodel.ConversationFieldMinSeq:                int64(0),
					chatmodel.ConversationFieldServerMaxSeq:          int64(0), // ★ 新建时固定写 0
					chatmodel.ConversationFieldCreateTime:            now,
					chatmodel.ConversationFieldIsMsgDestruct:         false,
					chatmodel.ConversationFieldMsgDestructTime:       int64(0),
					chatmodel.ConversationFieldLatestMsgDestructTime: time.Time{},
				},
				"$set": bson.M{
					"update_time": now, // 如无该字段可去掉
				},
			},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			log.Errorf(ctx, "upsert(insert-only) err: %v", err)
			return false, err
		}

		// 如果这次是“插入”，就保持 server_max_seq=0，不再前移
		if res.UpsertedCount > 0 {
			return true, nil
		}

		// Step 2: 已存在 → 只用 $max 前移（防回退），不触碰其它字段
		_, err = conv.Collection().UpdateOne(ctx,
			bson.M{
				chatmodel.ConversationFieldTenantID:       tenantID,
				chatmodel.ConversationFieldOwnerUserID:    owner,
				chatmodel.ConversationFieldConversationID: conversationID,
			},
			bson.M{
				"$max": bson.M{
					chatmodel.ConversationFieldServerMaxSeq: serverMaxSeq, // ★ 仅前移
				},
				"$set": bson.M{
					chatmodel.ConversationFieldUpdatedAt: now,
				},
			},
		)
		if err != nil {
			log.Errorf(ctx, "bump(server_max_seq) err: %v", err)
			return false, err
		}

		return false, nil
	}

	// 逐个确保
	cA, err := up(userA, userB)
	if err != nil {
		return false, false, err
	}
	cB, err := up(userB, userA)
	if err != nil {
		return cA, false, err
	}

	return cA, cB, nil
}

func EnsureIndexes(ctx context.Context) error {

	db := mgo.GetDB()
	seq := chatmodel.SeqConversation{}
	cov := chatmodel.Conversation{}
	msg := chatmodel.MessageModel{}
	rsb := chatmodel.ReadSparseBlock{}
	met := chatmodel.MentionIndex{}

	collections := map[string][]mongo.IndexModel{
		seq.GetTableName(): {{
			Keys: bson.D{{chatmodel.SeqConvFieldTenantID, 1},
				{chatmodel.SeqConvFieldConversationID, 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_tenant_conv"),
		}},
		cov.GetTableName(): {{
			Keys: bson.D{{chatmodel.ConversationFieldTenantID, 1},
				{chatmodel.ConversationFieldOwnerUserID, 1},
				{chatmodel.ConversationFieldConversationID, 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_user_conv"),
		}},
		msg.GetTableName(): {
			{
				Keys: bson.D{{chatmodel.MsgFieldTenantID, 1},
					{chatmodel.MsgFieldConversationID, 1},
					{chatmodel.MsgFieldSeq, 1}},
				Options: options.Index().SetUnique(true).SetName("ix_conv_seq"),
			},
			{
				Keys: bson.D{{chatmodel.MsgFieldTenantID, 1},
					{chatmodel.MsgFieldConversationID, 1},
					{chatmodel.MsgFieldSendID, 1},
					{chatmodel.MsgFieldSeq, 1}},
				Options: options.Index().SetName("ix_sender_seq"),
			},
		},
		rsb.GetTableName(): {{
			Keys: bson.D{{chatmodel.RSBFieldTenantID, 1},
				{chatmodel.RSBFieldConversationID, 1},
				{chatmodel.RSBFieldUserID, 1},
				{chatmodel.RSBFieldBlockStart, 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_sparse_block"),
		}},
		met.GetTableName(): {{
			Keys: bson.D{{chatmodel.MIFieldTenantID, 1},
				{chatmodel.MIFieldConversationID, 1},
				{chatmodel.MIFieldTargetUserID, 1},
				{chatmodel.MIFieldSeq, 1}},
			Options: options.Index().SetName("ix_mention_range"),
		}},
	}

	for collName, indexes := range collections {
		coll := db.Collection(collName)

		// 已有索引列表
		existing, err := coll.Indexes().ListSpecifications(ctx)
		if err != nil {
			return fmt.Errorf("list indexes for %s: %w", collName, err)
		}
		existingNames := make(map[string]struct{}, len(existing))
		for _, spec := range existing {
			existingNames[spec.Name] = struct{}{}
		}

		// 只创建不存在的
		for _, idx := range indexes {
			if idx.Options != nil && idx.Options.Name != nil {
				if _, ok := existingNames[*idx.Options.Name]; ok {
					continue // 已存在
				}
			}
			if _, err := coll.Indexes().CreateOne(ctx, idx); err != nil {
				return fmt.Errorf("create index %s on %s: %w", *idx.Options.Name, collName, err)
			}
		}
	}

	return nil
}
