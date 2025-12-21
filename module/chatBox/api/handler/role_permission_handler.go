package handler

import (
	"PProject/logger"
	"PProject/module/chatBox/service/dto"
	"PProject/module/chatBox/service/repo"
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RolePermissionHandler struct {
	RoleRepo       repo.RoleRepo
	PermissionRepo repo.PermissionRepo
	RPRepo         repo.RolePermissionRepo
}

func NewRolePermissionHandler(
	roleRepo repo.RoleRepo,
	permissionRepo repo.PermissionRepo,
	rpRepo repo.RolePermissionRepo,
) *RolePermissionHandler {
	return &RolePermissionHandler{
		RoleRepo:       roleRepo,
		PermissionRepo: permissionRepo,
		RPRepo:         rpRepo,
	}
}

// POST /tenants/:tenantId/roles/:roleId/permissions
func (h *RolePermissionHandler) BindBatch(c *gin.Context) {
	tenantId := c.Param("tenantId")
	roleId := c.Param("roleId")

	rid, err := primitive.ObjectIDFromHex(roleId)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid roleId"})
		return
	}

	var req dto.BindRolePermissionsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if len(req.PermissionIDs) == 0 {
		c.JSON(400, gin.H{"error": "permission_ids is empty"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 1) 校验 role 属于 tenant
	if _, err := h.RoleRepo.FindByID(ctx, tenantId, rid); err != nil {
		c.JSON(404, gin.H{"error": "role not found"})
		return
	}

	// 2) 校验 permissionId 合法（格式正确即可；是否存在可选）
	for _, pid := range req.PermissionIDs {
		if _, err := primitive.ObjectIDFromHex(pid); err != nil {
			c.JSON(400, gin.H{"error": "invalid permission_id: " + pid})
			return
		}
	}

	// 3) 幂等绑定
	for _, pid := range req.PermissionIDs {
		if err := h.RPRepo.BindUpsert(ctx, roleId, pid); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(200, gin.H{"ok": true})
}

// GET /tenants/:tenantId/roles/:roleId/permissions
func (h *RolePermissionHandler) ListByRole(c *gin.Context) {
	tenantId := c.Param("tenantId")
	roleId := c.Param("roleId")

	rid, err := primitive.ObjectIDFromHex(roleId)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid roleId"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 1) 校验 role 属于 tenant
	if _, err := h.RoleRepo.FindByID(ctx, tenantId, rid); err != nil {
		c.JSON(404, gin.H{"error": "role not found"})
		return
	}

	// 2) 查 role_permission 表拿 permission_ids
	permIDs, err := h.RPRepo.ListPermissionIDsByRole(ctx, roleId)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	logger.Infof("list permissions by role success: roleId=%s, permissionCount=%d, permissionIDs=%v", roleId, len(permIDs), permIDs)

	// 3) 批量查 permission 详情
	objIDs := make([]primitive.ObjectID, 0, len(permIDs))
	for _, pid := range permIDs {
		oid, _ := primitive.ObjectIDFromHex(pid)
		objIDs = append(objIDs, oid)
	}
	perms, err := h.PermissionRepo.FindByIDs(ctx, objIDs)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	logger.Infof(
		"[RBAC] permissions loaded: roleId=%s, permissionCount=%d, permissions=%+v",
		roleId,
		len(perms),
		perms,
	)

	resp := dto.RolePermissionsResp{
		RoleID:      roleId,
		Permissions: make([]dto.PermissionResp, 0, len(perms)),
	}
	for _, p := range perms {
		resp.Permissions = append(resp.Permissions, dto.PermissionResp{
			ID:   p.ID.Hex(),
			Code: p.Code,
			Name: p.Name,
			Desc: p.Desc,
		})
	}

	c.JSON(200, resp)
}

// DELETE /tenants/:tenantId/roles/:roleId/permissions/:permissionId
func (h *RolePermissionHandler) Unbind(c *gin.Context) {
	tenantId := c.Param("tenantId")
	roleId := c.Param("roleId")
	permissionId := c.Param("permissionId")

	rid, err := primitive.ObjectIDFromHex(roleId)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid roleId"})
		return
	}
	if _, err := primitive.ObjectIDFromHex(permissionId); err != nil {
		c.JSON(400, gin.H{"error": "invalid permissionId"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	// 校验 role 属于 tenant
	if _, err := h.RoleRepo.FindByID(ctx, tenantId, rid); err != nil {
		c.JSON(404, gin.H{"error": "role not found"})
		return
	}

	if err := h.RPRepo.Unbind(ctx, roleId, permissionId); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"ok": true})
}
