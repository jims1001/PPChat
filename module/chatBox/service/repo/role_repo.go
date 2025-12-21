package repo

import (
	"context"
	"time"

	"PProject/module/chatBox/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type RoleRepo struct{}

func NewRoleRepo() RoleRepo { return RoleRepo{} }

func (r RoleRepo) ExistsCode(ctx context.Context, tenantId, code string) (bool, error) {
	col := (&model.Role{}).Collection()
	n, err := col.CountDocuments(ctx, bson.M{"tenant_id": tenantId, "code": code})
	return n > 0, err
}

func (r RoleRepo) Insert(ctx context.Context, doc model.Role) error {
	_, err := (&model.Role{}).Collection().InsertOne(ctx, doc)
	return err
}

func (r RoleRepo) FindByID(ctx context.Context, tenantId string, id primitive.ObjectID) (*model.Role, error) {
	var out model.Role
	err := (&model.Role{}).Collection().FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantId}).Decode(&out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r RoleRepo) Patch(ctx context.Context, tenantId string, id primitive.ObjectID, set bson.M) error {
	set["updated_at"] = time.Now().UTC()
	_, err := (&model.Role{}).Collection().UpdateOne(ctx,
		bson.M{"_id": id, "tenant_id": tenantId},
		bson.M{"$set": set},
	)
	return err
}

func (r RoleRepo) Delete(ctx context.Context, tenantId string, id primitive.ObjectID) error {
	_, err := (&model.Role{}).Collection().DeleteOne(ctx, bson.M{"_id": id, "tenant_id": tenantId})
	return err
}

func (r RoleRepo) List(
	ctx context.Context,
	tenantId string,
	filter bson.M,
	page, pageSize int,
) ([]model.Role, int64, error) {
	col := (&model.Role{}).Collection()

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	// tenant 隔离
	if filter == nil {
		filter = bson.M{}
	}
	filter["tenant_id"] = tenantId

	// 可选：如果你做软删，可以在这里加：
	// filter["deleted_at"] = bson.M{"$exists": false}

	total, err := col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip := int64((page - 1) * pageSize)
	limit := int64(pageSize)

	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cur, err := col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	items := make([]model.Role, 0, pageSize)
	for cur.Next(ctx) {
		var x model.Role
		if err := cur.Decode(&x); err != nil {
			return nil, 0, err
		}
		items = append(items, x)
	}
	if err := cur.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
