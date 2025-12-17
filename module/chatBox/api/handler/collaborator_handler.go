package handler

import (
	"PProject/module/chatBox/service/dto"
	"PProject/module/chatBox/service/repo"
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

type CollaboratorHandler struct {
	Repo repo.AgentCollaboratorRepo
}

func NewCollaboratorHandler(repo repo.AgentCollaboratorRepo) *CollaboratorHandler {
	return &CollaboratorHandler{Repo: repo}
}

// POST /tenants/:tenantId/conversations/:conversationId/collaborators
func (h *CollaboratorHandler) Add(c *gin.Context) {
	tenantId := c.Param("tenantId")
	conversationId := c.Param("conversationId")

	var req dto.AddCollaboratorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Role == "" {
		req.Role = "collaborator"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	row, err := h.Repo.UpsertActive(ctx, tenantId, conversationId, req.AgentID, req.Role)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"id":              row.ID.Hex(),
		"tenant_id":       row.TenantID,
		"conversation_id": row.ConversationID,
		"agent_id":        row.AgentID,
		"role":            row.Role,
		"status":          row.Status,
		"joined_at":       row.JoinedAt.Format(time.RFC3339),
		"left_at": func() string {
			if row.LeftAt == nil {
				return ""
			}
			return row.LeftAt.Format(time.RFC3339)
		}(),
	})
}

// GET /tenants/:tenantId/conversations/:conversationId/collaborators
func (h *CollaboratorHandler) List(c *gin.Context) {
	tenantId := c.Param("tenantId")
	conversationId := c.Param("conversationId")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	rows, err := h.Repo.ListActive(ctx, tenantId, conversationId)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	out := make([]dto.CollaboratorResp, 0, len(rows))
	for _, r := range rows {
		item := dto.CollaboratorResp{
			ID:             r.ID.Hex(),
			TenantID:       r.TenantID,
			ConversationID: r.ConversationID,
			AgentID:        r.AgentID,
			Role:           r.Role,
			Status:         r.Status,
			JoinedAt:       r.JoinedAt.Format(time.RFC3339),
		}
		if r.LeftAt != nil {
			item.LeftAt = r.LeftAt.Format(time.RFC3339)
		}
		out = append(out, item)
	}

	c.JSON(200, gin.H{"items": out})
}

// DELETE /tenants/:tenantId/conversations/:conversationId/collaborators/:agentId
func (h *CollaboratorHandler) Remove(c *gin.Context) {
	tenantId := c.Param("tenantId")
	conversationId := c.Param("conversationId")
	agentId := c.Param("agentId")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := h.Repo.Remove(ctx, tenantId, conversationId, agentId); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
