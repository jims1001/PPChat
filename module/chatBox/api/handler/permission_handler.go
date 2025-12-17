package handler

import (
	"PProject/module/chatBox/model"
	"PProject/module/chatBox/service/dto"
	"PProject/module/chatBox/service/repo"
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PermissionHandler struct {
	Repo repo.PermissionRepo
}

func NewPermissionHandler(repo repo.PermissionRepo) *PermissionHandler {
	return &PermissionHandler{Repo: repo}
}

func toPermissionResp(p *model.Permission) dto.PermissionResp {
	return dto.PermissionResp{
		ID:   p.ID.Hex(),
		Code: p.Code,
		Name: p.Name,
		Desc: p.Desc,
	}
}

// POST /permissions
func (h *PermissionHandler) Create(c *gin.Context) {
	var req dto.CreatePermissionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	exists, err := h.Repo.ExistsCode(ctx, req.Code)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if exists {
		c.JSON(409, gin.H{"error": "permission code already exists"})
		return
	}

	doc := model.Permission{
		ID:   primitive.NewObjectID(),
		Code: req.Code,
		Name: req.Name,
		Desc: req.Desc,
	}

	if err := h.Repo.Insert(ctx, doc); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, toPermissionResp(&doc))
}

// GET /permissions?q=xxx&page=1&size=20
func (h *PermissionHandler) List(c *gin.Context) {
	var q dto.ListPermissionQuery
	_ = c.ShouldBindQuery(&q)
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Size <= 0 || q.Size > 200 {
		q.Size = 20
	}
	skip := (q.Page - 1) * q.Size
	limit := q.Size

	filter := bson.M{}
	if q.Code != "" {
		filter["code"] = q.Code
	}
	if q.Name != "" {
		filter["name"] = q.Name
	}
	if q.Q != "" {
		filter["$or"] = []bson.M{
			{"code": bson.M{"$regex": q.Q, "$options": "i"}},
			{"name": bson.M{"$regex": q.Q, "$options": "i"}},
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	items, total, err := h.Repo.List(ctx, filter, skip, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	out := make([]dto.PermissionResp, 0, len(items))
	for i := range items {
		out = append(out, toPermissionResp(&items[i]))
	}

	c.JSON(200, gin.H{
		"page":  q.Page,
		"size":  q.Size,
		"total": total,
		"items": out,
	})
}

// GET /permissions/:permissionId
func (h *PermissionHandler) Get(c *gin.Context) {
	pid := c.Param("permissionId")
	oid, err := primitive.ObjectIDFromHex(pid)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid permissionId"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	p, err := h.Repo.FindByID(ctx, oid)
	if err != nil {
		c.JSON(404, gin.H{"error": "permission not found"})
		return
	}
	c.JSON(200, toPermissionResp(p))
}

// PATCH /permissions/:permissionId
func (h *PermissionHandler) Patch(c *gin.Context) {
	pid := c.Param("permissionId")
	oid, err := primitive.ObjectIDFromHex(pid)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid permissionId"})
		return
	}

	var req dto.PatchPermissionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	set := bson.M{}
	if req.Code != nil {
		set["code"] = *req.Code
	}
	if req.Name != nil {
		set["name"] = *req.Name
	}
	if req.Desc != nil {
		set["desc"] = *req.Desc
	}
	if len(set) == 0 {
		c.JSON(400, gin.H{"error": "no fields to update"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	// 如果改 code，做唯一性校验
	if req.Code != nil {
		exists, err := h.Repo.ExistsCode(ctx, *req.Code)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if exists {
			c.JSON(409, gin.H{"error": "permission code already exists"})
			return
		}
	}

	// 可选：先确保存在
	if _, err := h.Repo.FindByID(ctx, oid); err != nil {
		c.JSON(404, gin.H{"error": "permission not found"})
		return
	}

	if err := h.Repo.Patch(ctx, oid, set); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// DELETE /permissions/:permissionId
func (h *PermissionHandler) Delete(c *gin.Context) {
	pid := c.Param("permissionId")
	oid, err := primitive.ObjectIDFromHex(pid)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid permissionId"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// 可选：先确保存在
	if _, err := h.Repo.FindByID(ctx, oid); err != nil {
		c.JSON(404, gin.H{"error": "permission not found"})
		return
	}

	if err := h.Repo.Delete(ctx, oid); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
