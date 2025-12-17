package repo

import (
	"context"
	"time"

	"PProject/module/chatBox/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
