package msgflow

import (
	chatmodel "PProject/module/chat/model"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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
		"tenant_id":       tenantID,
		"conversation_id": conversationID,
	}
	update := bson.M{
		"$setOnInsert": bson.M{
			"tenant_id":       tenantID,
			"conversation_id": conversationID,
			"max_seq":         int64(0),
			"min_seq":         int64(0),
			"issued_seq":      int64(0),
			"compact_wm":      int64(0),
			"epoch":           int32(0),
			"shard_key":       "",
			"create_time":     now,
		},
		"$set": bson.M{
			"update_time": now,
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
		"conversation_id": conversationID,
	}
	update := bson.M{
		"$max": bson.M{"max_seq": newMax},
		"$set": bson.M{"update_time": time.Now()},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).
		SetReturnDocument(options.After).
		SetProjection(bson.M{"max_seq": 1})

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

func normPair(a, b string) (lo, hi string) {
	if a <= b {
		return a, b
	}
	return b, a
}

// 单聊的统一会话ID：p2p:min_max
func buildP2PConvID(a, b string) string {
	lo, hi := normPair(a, b)
	return "p2p:" + lo + "_" + hi
}

// EnsureTwoSidesByKnownConvID
// 已知 covID：如果两条都存在 -> 只更新 server_max_seq；否则 -> 创建缺失的并刷新影子。
// 返回 createdA/createdB 表示 A/B 是否在本次被创建。
func EnsureTwoSidesByKnownConvID(ctx context.Context, tenantID string, conversationID string, // 已知 covId（p2p:min_max / grp:xxx）
	chatType int32,                                                                           // 1=单聊 2=群聊 ...
	userA string,                                                                             // A 作为 owner
	userB string,                                                                             // B 作为 owner
	serverMaxSeq int64,                                                                       // 本次影子水位（没有就传 0）
) (createdA bool, createdB bool, err error) {

	conv := chatmodel.Conversation{}
	// 1) 先看是否两条都在
	cnt, err := conv.Collection().CountDocuments(ctx, bson.M{
		"tenant_id":       tenantID,
		"conversation_id": conversationID,
		"owner_user_id":   bson.M{"$in": []string{userA, userB}},
	})
	if err != nil {
		return false, false, err
	}

	now := time.Now()

	// 2) 两条都存在：只更新 server_max_seq（用 $max 防止回退）
	if cnt == 2 {
		_, err = conv.Collection().UpdateMany(ctx,
			bson.M{
				"tenant_id":       tenantID,
				"conversation_id": conversationID,
				"owner_user_id":   bson.M{"$in": []string{userA, userB}},
			},
			bson.M{
				"$max": bson.M{"server_max_seq": serverMaxSeq},
				"$set": bson.M{"update_time": now},
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
				"tenant_id":       tenantID,
				"owner_user_id":   owner,
				"conversation_id": conversationID,
			},
			bson.M{
				"$setOnInsert": bson.M{
					"tenant_id":                tenantID,
					"owner_user_id":            owner,
					"conversation_id":          conversationID,
					"conversation_type":        chatType,
					"user_id":                  peer, // 单聊对端；群聊可留空
					"group_id":                 "",
					"recv_msg_opt":             int32(0),
					"is_pinned":                false,
					"is_private_chat":          false,
					"burn_duration":            int32(0),
					"group_at_type":            int32(0),
					"attached_info":            "",
					"ex":                       "",
					"max_seq":                  int64(0), // 个人读游标
					"min_seq":                  int64(0),
					"server_max_seq":           int64(0), // ★ 新建时固定写 0
					"create_time":              now,
					"is_msg_destruct":          false,
					"msg_destruct_time":        int64(0),
					"latest_msg_destruct_time": time.Time{},
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
				"tenant_id":       tenantID,
				"owner_user_id":   owner,
				"conversation_id": conversationID,
			},
			bson.M{
				"$max": bson.M{
					"server_max_seq": serverMaxSeq, // ★ 仅前移
				},
				"$set": bson.M{
					"update_time": now,
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
