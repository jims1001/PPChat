package repo

import (
	"context"
	"time"

	"PProject/module/chatBox/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AgentRepo struct{}

func NewAgentRepo() AgentRepo { return AgentRepo{} }

func (r AgentRepo) ExistsEmail(ctx context.Context, tenantId, email string) (bool, error) {
	col := (&model.Agent{}).Collection()
	n, err := col.CountDocuments(ctx, bson.M{"tenant_id": tenantId, "email": email})
	return n > 0, err
}

func (r AgentRepo) Insert(ctx context.Context, doc model.Agent) error {
	_, err := (&model.Agent{}).Collection().InsertOne(ctx, doc)
	return err
}

func (r AgentRepo) FindByID(ctx context.Context, tenantId string, id primitive.ObjectID) (*model.Agent, error) {
	var out model.Agent
	err := (&model.Agent{}).Collection().FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantId}).Decode(&out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r AgentRepo) Patch(ctx context.Context, tenantId string, id primitive.ObjectID, set bson.M) error {
	set["updated_at"] = time.Now().UTC()
	_, err := (&model.Agent{}).Collection().UpdateOne(ctx,
		bson.M{"_id": id, "tenant_id": tenantId},
		bson.M{"$set": set},
	)
	return err
}

func (r AgentRepo) List(
	ctx context.Context,
	tenantId string,
	filter bson.M,
	skip, limit int64,
) ([]model.Agent, int64, error) {

	col := (&model.Agent{}).Collection()

	// copy filter to avoid mutating caller's map
	f := bson.M{}
	for k, v := range filter {
		f[k] = v
	}
	f["tenant_id"] = tenantId

	total, err := col.CountDocuments(ctx, f)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cur, err := col.Find(ctx, f, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var items []model.Agent
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
