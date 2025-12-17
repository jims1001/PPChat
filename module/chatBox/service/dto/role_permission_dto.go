package dto

type BindRolePermissionsReq struct {
	PermissionIDs []string `json:"permission_ids" binding:"required"`
}

type RolePermissionsResp struct {
	RoleID      string           `json:"role_id"`
	Permissions []PermissionResp `json:"permissions"`
}
