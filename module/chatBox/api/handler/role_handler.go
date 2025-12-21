package handler

import (
	"PProject/module/chatBox/model"
	"PProject/module/chatBox/service/dto"
	"PProject/module/chatBox/service/repo"
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RoleHandler struct {
	Repo repo.RoleRepo
}

func NewRoleHandler(roleRepo repo.RoleRepo) *RoleHandler {
	return &RoleHandler{
		Repo: roleRepo,
	}
}

func (h *RoleHandler) Create(c *gin.Context) {
	tenantId := c.Param("tenantId")

	var req dto.CreateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	exists, err := h.Repo.ExistsCode(ctx, tenantId, req.Code)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if exists {
		c.JSON(409, gin.H{"error": "role code already exists"})
		return
	}

	now := time.Now().UTC()
	doc := model.Role{
		ID:        primitive.NewObjectID(),
		TenantID:  tenantId,
		AccountID: req.AccountID,
		Name:      req.Name,
		Code:      req.Code,
		Desc:      req.Desc,
		BuiltIn:   req.BuiltIn,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.Repo.Insert(ctx, doc); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, dto.RoleResp{
		ID:        doc.ID.Hex(),
		TenantID:  doc.TenantID,
		AccountID: doc.AccountID,
		Name:      doc.Name,
		Code:      doc.Code,
		Desc:      doc.Desc,
		BuiltIn:   doc.BuiltIn,
		Status:    doc.Status,
	})
}

func (h *RoleHandler) Patch(c *gin.Context) {
	tenantId := c.Param("tenantId")
	roleId := c.Param("roleId")
	rid, err := primitive.ObjectIDFromHex(roleId)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid roleId"})
		return
	}

	var req dto.PatchRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	old, err := h.Repo.FindByID(ctx, tenantId, rid)
	if err != nil {
		c.JSON(404, gin.H{"error": "role not found"})
		return
	}
	if old.BuiltIn {
		c.JSON(403, gin.H{"error": "built-in role is not editable"})
		return
	}

	set := bson.M{}
	if req.Name != nil {
		set["name"] = *req.Name
	}
	if req.Desc != nil {
		set["desc"] = *req.Desc
	}
	if req.Status != nil {
		set["status"] = *req.Status
	}

	if err := h.Repo.Patch(ctx, tenantId, rid, set); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (h *RoleHandler) List(c *gin.Context) {
	tenantId := c.Param("tenantId")

	// 可选：分页参数（默认 page=1, page_size=20）
	page := 1
	pageSize := 20
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		} else {
			c.JSON(400, gin.H{"error": "invalid page"})
			return
		}
	}
	if v := c.Query("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			pageSize = n
		} else {
			c.JSON(400, gin.H{"error": "invalid page_size"})
			return
		}
	}

	// 可选：简单过滤（按 status / keyword）
	filter := bson.M{}
	if status := c.Query("status"); status != "" {
		filter["status"] = status
	}
	if kw := strings.TrimSpace(c.Query("q")); kw != "" {
		// name/code 模糊搜索（Mongo 正则）
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": kw, "$options": "i"}},
			{"code": bson.M{"$regex": kw, "$options": "i"}},
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	items, total, err := h.Repo.List(ctx, tenantId, filter, page, pageSize)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"items":     items,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

func (h *RoleHandler) Get(c *gin.Context) {
	tenantId := c.Param("tenantId")
	roleId := c.Param("roleId")

	rid, err := primitive.ObjectIDFromHex(roleId)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid roleId"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	role, err := h.Repo.FindByID(ctx, tenantId, rid)
	if err != nil {
		c.JSON(404, gin.H{"error": "role not found"})
		return
	}

	c.JSON(200, gin.H{"item": role})
}

func (h *RoleHandler) Delete(c *gin.Context) {
	tenantId := c.Param("tenantId")
	roleId := c.Param("roleId")

	rid, err := primitive.ObjectIDFromHex(roleId)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid roleId"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// 先查再删：用于 built-in 保护 & 404 语义一致
	old, err := h.Repo.FindByID(ctx, tenantId, rid)
	if err != nil {
		c.JSON(404, gin.H{"error": "role not found"})
		return
	}
	if old.BuiltIn {
		c.JSON(403, gin.H{"error": "built-in role is not deletable"})
		return
	}

	if err := h.Repo.Delete(ctx, tenantId, rid); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"ok": true})
}
