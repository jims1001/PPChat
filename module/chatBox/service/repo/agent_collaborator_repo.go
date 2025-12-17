package repo

import (
	"context"
	"time"

	"PProject/module/chatBox/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AgentCollaboratorRepo struct{}

func NewAgentCollaboratorRepo() AgentCollaboratorRepo { return AgentCollaboratorRepo{} }

// Upsert：同 (tenant, conversation, agent) 唯一；重复 Add 不报错，直接保持 active
func (r AgentCollaboratorRepo) UpsertActive(ctx context.Context, tenantId, conversationId, agentId, role string) (*model.AgentCollaborator, error) {
	col := (&model.AgentCollaborator{}).Collection()
	now := time.Now().UTC()

	filter := bson.M{
		"tenant_id":       tenantId,
		"conversation_id": conversationId,
		"agent_id":        agentId,
	}

	update := bson.M{
		"$set": bson.M{
			"tenant_id":       tenantId,
			"conversation_id": conversationId,
			"agent_id":        agentId,
			"role":            role,
			"status":          "active",
			"left_at":         nil,
			// 注意：joined_at 的语义一般是“首次加入时间”，这里不强制刷新
		},
		"$setOnInsert": bson.M{
			"_id":       primitive.NewObjectID(),
			"joined_at": now,
		},
	}

	_, err := col.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return nil, err
	}

	return r.FindOne(ctx, tenantId, conversationId, agentId)
}

func (r AgentCollaboratorRepo) FindOne(ctx context.Context, tenantId, conversationId, agentId string) (*model.AgentCollaborator, error) {
	var out model.AgentCollaborator
	err := (&model.AgentCollaborator{}).Collection().FindOne(ctx, bson.M{
		"tenant_id":       tenantId,
		"conversation_id": conversationId,
		"agent_id":        agentId,
	}).Decode(&out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r AgentCollaboratorRepo) ListActive(
	ctx context.Context,
	tenantId, conversationId string,
) ([]model.AgentCollaborator, error) {

	col := (&model.AgentCollaborator{}).Collection()

	filter := bson.M{
		"tenant_id":       tenantId,
		"conversation_id": conversationId,
		"status":          "active",
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "joined_at", Value: 1}})

	cur, err := col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []model.AgentCollaborator
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r AgentCollaboratorRepo) Remove(ctx context.Context, tenantId, conversationId, agentId string) error {
	now := time.Now().UTC()
	_, err := (&model.AgentCollaborator{}).Collection().UpdateOne(ctx,
		bson.M{"tenant_id": tenantId, "conversation_id": conversationId, "agent_id": agentId},
		bson.M{"$set": bson.M{"status": "inactive", "left_at": now}},
	)
	return err
}
