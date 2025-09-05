package msgflow

import (
	chatmodel "PProject/module/chat/model"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const ConvTypeP2P int = 100
const ConvTypeGroup int = 200
const convTypeChannel int = 300

// EnsureP2PForOwner
// 语义：确保 (tenantID, ownerUserID, peerUserID, chatType) 这一条单聊会话存在；
// - 存在：直接返回现存的 conversation_id
// - 不存在：创建后返回新建的 conversation_id
func EnsureP2PForOwner(
	ctx context.Context,
	tenantID string,
	ownerUserID string,
	peerUserID string,
	chatType int32, // 建议用 int32 与 bson 对齐
) (conversationID string, created bool, err error) {

	cov := chatmodel.Conversation{}
	c := cov.Collection()

	// 规则：单聊的 ConversationID = 对端 userID（你也可以换成前缀规则）
	conversationID = peerUserID

	// 1) 先查是否已存在（按 tenant + owner + type + user_id）
	//    如果存在，直接返回已存的 conversation_id（以数据库为准）
	var existed struct {
		ConversationID string `bson:"conversation_id"`
	}
	findFilter := bson.M{
		"tenant_id":         tenantID,
		"owner_user_id":     ownerUserID,
		"conversation_type": chatType,
		"user_id":           peerUserID,
	}
	err = c.FindOne(ctx, findFilter, options.FindOne().
		SetProjection(bson.M{"conversation_id": 1, "_id": 0}),
	).Decode(&existed)
	if err == nil {
		return existed.ConversationID, false, nil
	}
	if err != mongo.ErrNoDocuments {
		return "", false, err
	}

	// 2) 不存在则创建（并发安全：用 upsert；filter 仍用上面的四元组）
	now := time.Now()
	upsertFilter := findFilter // 与查询一致，确保“同一组合”唯一命中
	update := bson.M{
		"$setOnInsert": bson.M{
			"tenant_id":                tenantID,
			"owner_user_id":            ownerUserID,
			"conversation_type":        chatType,
			"conversation_id":          conversationID, // 记录会话ID
			"user_id":                  peerUserID,     // 单聊对端
			"group_id":                 "",
			"recv_msg_opt":             int32(0),
			"is_pinned":                false,
			"is_private_chat":          false,
			"burn_duration":            int32(0),
			"group_at_type":            int32(0),
			"attached_info":            "",
			"ex":                       "",
			"max_seq":                  int64(0),
			"min_seq":                  int64(0),
			"create_time":              now,
			"is_msg_destruct":          false,
			"msg_destruct_time":        int64(0),
			"latest_msg_destruct_time": time.Time{},
		},
	}
	res, err := c.UpdateOne(ctx, upsertFilter, update, options.Update().SetUpsert(true))
	if err != nil {
		return "", false, err
	}
	// 3) 判断是否新建。若不是新建，说明并发下被别人抢先插入了 → 再查一次拿 covID
	if res.UpsertedCount == 0 {
		// 并发已插入，读回 covID
		err = c.FindOne(ctx, findFilter,
			options.FindOne().SetProjection(bson.M{"conversation_id": 1, "_id": 0}),
		).Decode(&existed)
		if err != nil {
			return "", false, err
		}
		return existed.ConversationID, false, nil
	}
	return conversationID, true, nil
}
