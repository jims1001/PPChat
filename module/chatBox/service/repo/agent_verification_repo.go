package repo

import (
	"context"
	"time"

	"PProject/module/chatBox/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AgentVerificationRepo struct{}

// 插入 invite 记录
func (r AgentVerificationRepo) InsertInvite(ctx context.Context, doc model.AgentVerification) error {
	_, err := (&model.AgentVerification{}).Collection().InsertOne(ctx, doc)
	return err
}

// 通过 token_hash 找 invite（未过期、未使用）
func (r AgentVerificationRepo) FindValidInviteByHash(ctx context.Context, tenantId, tokenHash string, now time.Time) (*model.AgentVerification, error) {
	var out model.AgentVerification
	err := (&model.AgentVerification{}).Collection().FindOne(ctx, bson.M{
		"tenant_id":  tenantId,
		"purpose":    "invite",
		"token_hash": tokenHash,
		"expires_at": bson.M{"$gt": now},
		"used_at":    bson.M{"$exists": false},
	}).Decode(&out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r AgentVerificationRepo) MarkUsed(ctx context.Context, tenantId string, id primitive.ObjectID, usedAt time.Time) error {
	_, err := (&model.AgentVerification{}).Collection().UpdateOne(ctx,
		bson.M{"_id": id, "tenant_id": tenantId},
		bson.M{"$set": bson.M{"used_at": usedAt}},
	)
	return err
}

// 可选：把某个 agent 未用 invite 全部作废（你也可以不做，直接插新记录）
func (r AgentVerificationRepo) InvalidateInvitesForAgent(ctx context.Context, tenantId, agentId string, now time.Time) error {
	_, err := (&model.AgentVerification{}).Collection().UpdateMany(ctx,
		bson.M{
			"tenant_id":  tenantId,
			"agent_id":   agentId,
			"purpose":    "invite",
			"used_at":    bson.M{"$exists": false},
			"expires_at": bson.M{"$gt": now},
		},
		bson.M{"$set": bson.M{"used_at": now}},
	)
	return err
}
