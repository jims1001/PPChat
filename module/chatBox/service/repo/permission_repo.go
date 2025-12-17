package repo

import (
	"context"

	"PProject/module/chatBox/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PermissionRepo struct{}

func NewPermissionRepo() PermissionRepo { return PermissionRepo{} }

func (r PermissionRepo) ExistsCode(ctx context.Context, code string) (bool, error) {
	col := (&model.Permission{}).Collection()
	n, err := col.CountDocuments(ctx, bson.M{"code": code})
	return n > 0, err
}

func (r PermissionRepo) Insert(ctx context.Context, doc model.Permission) error {
	_, err := (&model.Permission{}).Collection().InsertOne(ctx, doc)
	return err
}

func (r PermissionRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*model.Permission, error) {
	var out model.Permission
	err := (&model.Permission{}).Collection().FindOne(ctx, bson.M{"_id": id}).Decode(&out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r PermissionRepo) Patch(ctx context.Context, id primitive.ObjectID, set bson.M) error {
	_, err := (&model.Permission{}).Collection().UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": set},
	)
	return err
}

func (r PermissionRepo) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := (&model.Permission{}).Collection().DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r PermissionRepo) List(
	ctx context.Context,
	filter bson.M,
	skip, limit int64,
) ([]model.Permission, int64, error) {

	col := (&model.Permission{}).Collection()

	// 可选：避免修改外部 filter
	f := bson.M{}
	for k, v := range filter {
		f[k] = v
	}

	total, err := col.CountDocuments(ctx, f)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(bson.D{{Key: "code", Value: 1}})

	cur, err := col.Find(ctx, f, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var items []model.Permission
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r PermissionRepo) FindByIDs(ctx context.Context, ids []primitive.ObjectID) ([]model.Permission, error) {
	if len(ids) == 0 {
		return []model.Permission{}, nil
	}
	col := (&model.Permission{}).Collection()

	cur, err := col.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []model.Permission
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
