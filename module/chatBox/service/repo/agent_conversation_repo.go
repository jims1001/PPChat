package repo

import (
	"context"
	"time"

	"PProject/module/chatBox/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AgentConversationRepo struct{}

func NewAgentConversationRepo() AgentConversationRepo { return AgentConversationRepo{} }

func (r AgentConversationRepo) UpsertAssignee(ctx context.Context, tenantId, conversationId string, set bson.M) (*model.AgentConversation, error) {
	col := (&model.AgentConversation{}).Collection()
	now := time.Now().UTC()

	filter := bson.M{"tenant_id": tenantId, "conversation_id": conversationId, "role": "assignee"}

	updateSet := bson.M{
		"tenant_id":       tenantId,
		"conversation_id": conversationId,
		"role":            "assignee",
		"status":          "active",
		"joined_at":       now,
		"left_at":         nil,
		"updated_at":      now,
	}
	for k, v := range set {
		updateSet[k] = v
	}

	update := bson.M{
		"$set": updateSet,
		"$setOnInsert": bson.M{
			"_id":        primitive.NewObjectID(),
			"created_at": now,
		},
	}

	_, err := col.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return nil, err
	}
	return r.FindAssignee(ctx, tenantId, conversationId)
}

func (r AgentConversationRepo) FindAssignee(ctx context.Context, tenantId, conversationId string) (*model.AgentConversation, error) {
	var out model.AgentConversation
	err := (&model.AgentConversation{}).Collection().FindOne(ctx, bson.M{
		"tenant_id":       tenantId,
		"conversation_id": conversationId,
		"role":            "assignee",
	}).Decode(&out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r AgentConversationRepo) RemoveAssignee(ctx context.Context, tenantId, conversationId, agentId string) error {
	now := time.Now().UTC()
	_, err := (&model.AgentConversation{}).Collection().UpdateOne(ctx,
		bson.M{
			"tenant_id":       tenantId,
			"conversation_id": conversationId,
			"agent_id":        agentId,
			"role":            "assignee",
		},
		bson.M{"$set": bson.M{"status": "inactive", "left_at": now, "updated_at": now}},
	)
	return err
}
