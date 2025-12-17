package repo

import (
	"context"

	"PProject/module/chatBox/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type RolePermissionRepo struct{}

func NewRolePermissionRepo() RolePermissionRepo { return RolePermissionRepo{} }

// 幂等绑定：存在则不变，不存在则插入
func (r RolePermissionRepo) BindUpsert(
	ctx context.Context,
	roleId, permissionId string,
) error {

	col := (&model.RolePermission{}).Collection()

	_, err := col.UpdateOne(
		ctx,
		bson.M{
			"role_id":       roleId,
			"permission_id": permissionId,
		},
		bson.M{
			"$setOnInsert": bson.M{
				"_id":           primitive.NewObjectID(),
				"role_id":       roleId,
				"permission_id": permissionId,
			},
		},
		options.Update().SetUpsert(true), // ✅ 正确
	)

	return err
}

func (r RolePermissionRepo) Unbind(ctx context.Context, roleId, permissionId string) error {
	_, err := (&model.RolePermission{}).Collection().DeleteOne(ctx, bson.M{
		"role_id":       roleId,
		"permission_id": permissionId,
	})
	return err
}

// 返回 role 绑定的 permissionId 列表
func (r RolePermissionRepo) ListPermissionIDsByRole(ctx context.Context, roleId string) ([]string, error) {
	col := (&model.RolePermission{}).Collection()

	cur, err := col.Find(ctx, bson.M{"role_id": roleId})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	type rp struct {
		PermissionID string `bson:"permission_id"`
	}
	var rows []rp
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(rows))
	for _, x := range rows {
		if x.PermissionID != "" {
			out = append(out, x.PermissionID)
		}
	}
	return out, nil
}
