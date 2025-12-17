package handler

import (
	"PProject/module/chatBox/model"
	"PProject/module/chatBox/service/dto"
	"PProject/module/chatBox/service/repo"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AgentHandler struct {
	AgentRepo repo.AgentRepo
	VerRepo   repo.AgentVerificationRepo

	// 可选：你自己的邮件服务
	// Mailer MailService
}

func NewAgentHandler(agentRepo repo.AgentRepo) *AgentHandler {
	return &AgentHandler{AgentRepo: agentRepo}
}

func randTokenHex(bytesLen int) (raw string, hash string, err error) {
	b := make([]byte, bytesLen)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b) // 2*bytesLen
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return raw, hash, nil
}

func toAgentResp(a *model.Agent) dto.AgentResp {
	return dto.AgentResp{
		ID:        a.ID.Hex(),
		TenantID:  a.TenantID,
		AccountID: a.AccountID,
		Name:      a.Name,
		Role:      a.Role,
		Email:     a.Email,
		Status:    a.Status,
		TeamIDs:   a.TeamIDs,
	}
}

// POST /tenants/:tenantId/agents
func (h *AgentHandler) CreateAgent(c *gin.Context) {
	tenantId := c.Param("tenantId")

	var req dto.CreateAgentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	exists, err := h.AgentRepo.ExistsEmail(ctx, tenantId, req.Email)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if exists {
		c.JSON(409, gin.H{"error": "email already exists"})
		return
	}

	now := time.Now().UTC()
	status := 1
	if req.Status != nil {
		status = *req.Status
	}

	agent := model.Agent{
		ID:        primitive.NewObjectID(),
		TenantID:  tenantId,
		AccountID: req.AccountID,
		Name:      req.Name,
		Role:      req.Role,
		Email:     req.Email,
		Status:    status,
		TeamIDs:   req.TeamIDs,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.AgentRepo.Insert(ctx, agent); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// 生成邀请（如果你想“创建即邀请”）
	raw, tokenHash, err := randTokenHex(24) // raw 长度 48
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	expiresAt := now.Add(48 * time.Hour)

	ver := model.AgentVerification{
		ID:        primitive.NewObjectID(),
		TenantID:  tenantId,
		AccountID: req.AccountID,
		AgentID:   agent.ID.Hex(),
		Email:     req.Email,
		Purpose:   "invite",
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		UsedAt:    nil,
		CreatedBy: req.CreatedBy,
		CreatedAt: now,
	}
	if ver.CreatedBy == "" {
		ver.CreatedBy = req.AccountID
	}

	if err := h.VerRepo.InsertInvite(ctx, ver); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// TODO: 这里调用 Mailer 发送邀请链接（生产环境不返回 token）
	// link := "https://yourhost/agents/accept?token=" + raw

	resp := dto.CreateAgentResp{
		Agent: toAgentResp(&agent),
		Invite: &dto.InviteInfoResp{
			Token:     raw, // 生产环境建议去掉
			ExpiresAt: expiresAt.Format(time.RFC3339),
		},
	}
	c.JSON(200, resp)
}

// GET /tenants/:tenantId/agents
func (h *AgentHandler) ListAgents(c *gin.Context) {
	tenantId := c.Param("tenantId")

	var q dto.ListAgentsQuery
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
	if q.Role != "" {
		filter["role"] = q.Role
	}
	if q.Status != nil {
		filter["status"] = *q.Status
	}
	if q.TeamID != "" {
		filter["team_ids"] = q.TeamID // team_ids 数组包含
	}
	if q.Email != "" {
		filter["email"] = bson.M{"$regex": q.Email, "$options": "i"}
	}
	if q.Name != "" {
		filter["name"] = bson.M{"$regex": q.Name, "$options": "i"}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	items, total, err := h.AgentRepo.List(ctx, tenantId, filter, skip, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	out := make([]dto.AgentResp, 0, len(items))
	for i := range items {
		out = append(out, toAgentResp(&items[i]))
	}

	c.JSON(200, gin.H{
		"page":  q.Page,
		"size":  q.Size,
		"total": total,
		"items": out,
	})
}

// GET /tenants/:tenantId/agents/:agentId
func (h *AgentHandler) GetAgent(c *gin.Context) {
	tenantId := c.Param("tenantId")
	agentId := c.Param("agentId")
	oid, err := primitive.ObjectIDFromHex(agentId)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid agentId"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	a, err := h.AgentRepo.FindByID(ctx, tenantId, oid)
	if err != nil {
		c.JSON(404, gin.H{"error": "agent not found"})
		return
	}
	c.JSON(200, toAgentResp(a))
}

// PATCH /tenants/:tenantId/agents/:agentId
func (h *AgentHandler) PatchAgent(c *gin.Context) {
	tenantId := c.Param("tenantId")
	agentId := c.Param("agentId")
	oid, err := primitive.ObjectIDFromHex(agentId)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid agentId"})
		return
	}

	var req dto.PatchAgentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	set := bson.M{}
	if req.Name != nil {
		set["name"] = *req.Name
	}
	if req.Role != nil {
		set["role"] = *req.Role
	}
	if req.Status != nil {
		set["status"] = *req.Status
	}
	if req.TeamIDs != nil {
		set["team_ids"] = req.TeamIDs
	}
	if len(set) == 0 {
		c.JSON(400, gin.H{"error": "no fields to update"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// 可选：先查存在性
	if _, err := h.AgentRepo.FindByID(ctx, tenantId, oid); err != nil {
		c.JSON(404, gin.H{"error": "agent not found"})
		return
	}

	if err := h.AgentRepo.Patch(ctx, tenantId, oid, set); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// POST /tenants/:tenantId/agents/:agentId/resend-invite
func (h *AgentHandler) ResendInvite(c *gin.Context) {
	tenantId := c.Param("tenantId")
	agentId := c.Param("agentId")
	oid, err := primitive.ObjectIDFromHex(agentId)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid agentId"})
		return
	}

	var req dto.ResendInviteReq
	_ = c.ShouldBindJSON(&req)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	a, err := h.AgentRepo.FindByID(ctx, tenantId, oid)
	if err != nil {
		c.JSON(404, gin.H{"error": "agent not found"})
		return
	}

	now := time.Now().UTC()

	// 可选：作废旧 invite
	_ = h.VerRepo.InvalidateInvitesForAgent(ctx, tenantId, agentId, now)

	raw, tokenHash, err := randTokenHex(24)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	expiresAt := now.Add(48 * time.Hour)

	ver := model.AgentVerification{
		ID:        primitive.NewObjectID(),
		TenantID:  tenantId,
		AccountID: a.AccountID,
		AgentID:   agentId,
		Email:     a.Email,
		Purpose:   "invite",
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		CreatedBy: req.CreatedBy,
		CreatedAt: now,
	}
	if ver.CreatedBy == "" {
		ver.CreatedBy = a.AccountID
	}

	if err := h.VerRepo.InsertInvite(ctx, ver); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// TODO: Mailer send link with raw token
	c.JSON(200, gin.H{
		"ok": true,
		"invite": dto.InviteInfoResp{
			Token:     raw, // 生产环境建议不返回
			ExpiresAt: expiresAt.Format(time.RFC3339),
		},
	})
}

// POST /tenants/:tenantId/agents/invites/accept
func (h *AgentHandler) AcceptInvite(c *gin.Context) {
	tenantId := c.Param("tenantId")

	var req dto.AcceptInviteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	sum := sha256.Sum256([]byte(req.Token))
	tokenHash := hex.EncodeToString(sum[:])

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	now := time.Now().UTC()
	ver, err := h.VerRepo.FindValidInviteByHash(ctx, tenantId, tokenHash, now)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid or expired token"})
		return
	}

	// 标记 used
	_ = h.VerRepo.MarkUsed(ctx, tenantId, ver.ID, now)

	// 激活 agent（你 Agent.Status 是 1/0，这里置 1）
	aid, err := primitive.ObjectIDFromHex(ver.AgentID)
	if err == nil {
		_ = h.AgentRepo.Patch(ctx, tenantId, aid, bson.M{"status": 1})
	}

	c.JSON(200, gin.H{"ok": true})
}
