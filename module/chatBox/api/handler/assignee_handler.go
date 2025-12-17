package handler

import (
	"PProject/module/chatBox/service/dto"
	"PProject/module/chatBox/service/repo"
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

type AssigneeHandler struct {
	Repo repo.AgentConversationRepo
}

func NewAssigneeHandler(repo repo.AgentConversationRepo) *AssigneeHandler {
	return &AssigneeHandler{Repo: repo}
}

func toAssigneeResp(m *repoAgentConversationView) dto.AssigneeResp {
	// 这个 helper 下面会用实际 model 转换；为了不引入 model 包在这里做一层 view
	return dto.AssigneeResp{
		ID:             m.ID,
		TenantID:       m.TenantID,
		ConversationID: m.ConversationID,
		AgentID:        m.AgentID,
		ContactID:      m.ContactID,
		InboxID:        m.InboxID,
		TeamID:         m.TeamID,
		Priority:       m.Priority,
		TagIDs:         m.TagIDs,
		Status:         m.Status,
		JoinedAt:       m.JoinedAt,
		LeftAt:         m.LeftAt,
	}
}

// 为了让 Handler 不直接依赖 model（你若不介意可直接用 model.AgentConversation）
type repoAgentConversationView struct {
	ID             string
	TenantID       string
	ConversationID string
	AgentID        string
	ContactID      string
	InboxID        string
	TeamID         string
	Priority       string
	TagIDs         []string
	Status         string
	JoinedAt       string
	LeftAt         string
}

func fromModel(m interface{}) *repoAgentConversationView {
	// 这里保持简单：直接在 handler 引 model 更直观
	return nil
}

// POST /tenants/:tenantId/conversations/:conversationId/assignee
func (h *AssigneeHandler) Set(c *gin.Context) {
	tenantId := c.Param("tenantId")
	conversationId := c.Param("conversationId")

	var req dto.SetAssigneeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// priority 校验可选（强约束的话就校验 oneof）
	if req.Priority == "" {
		req.Priority = "normal"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	set := bson.M{
		"agent_id":   req.AgentID,
		"contact_id": req.ContactID,
		"inbox_id":   req.InboxID,
		"team_id":    req.TeamID,
		"priority":   req.Priority,
		"tag_ids":    req.TagIDs,
	}
	assignee, err := h.Repo.UpsertAssignee(ctx, tenantId, conversationId, set)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"id":              assignee.ID.Hex(),
		"tenant_id":       assignee.TenantID,
		"conversation_id": assignee.ConversationID,
		"agent_id":        assignee.AgentID,
		"contact_id":      assignee.ContactID,
		"inbox_id":        assignee.InboxID,
		"team_id":         assignee.TeamID,
		"priority":        assignee.Priority,
		"tag_ids":         assignee.TagIDs,
		"status":          assignee.Status,
		"joined_at":       assignee.JoinedAt.Format(time.RFC3339),
		"left_at": func() string {
			if assignee.LeftAt == nil {
				return ""
			}
			return assignee.LeftAt.Format(time.RFC3339)
		}(),
	})
}

// GET /tenants/:tenantId/conversations/:conversationId/assignee
func (h *AssigneeHandler) Get(c *gin.Context) {
	tenantId := c.Param("tenantId")
	conversationId := c.Param("conversationId")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	assignee, err := h.Repo.FindAssignee(ctx, tenantId, conversationId)
	if err != nil {
		c.JSON(404, gin.H{"error": "assignee not found"})
		return
	}

	c.JSON(200, gin.H{
		"id":              assignee.ID.Hex(),
		"tenant_id":       assignee.TenantID,
		"conversation_id": assignee.ConversationID,
		"agent_id":        assignee.AgentID,
		"contact_id":      assignee.ContactID,
		"inbox_id":        assignee.InboxID,
		"team_id":         assignee.TeamID,
		"priority":        assignee.Priority,
		"tag_ids":         assignee.TagIDs,
		"status":          assignee.Status,
		"joined_at":       assignee.JoinedAt.Format(time.RFC3339),
		"left_at": func() string {
			if assignee.LeftAt == nil {
				return ""
			}
			return assignee.LeftAt.Format(time.RFC3339)
		}(),
	})
}

// DELETE /tenants/:tenantId/conversations/:conversationId/assignee/:agentId
func (h *AssigneeHandler) Remove(c *gin.Context) {
	tenantId := c.Param("tenantId")
	conversationId := c.Param("conversationId")
	agentId := c.Param("agentId")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := h.Repo.RemoveAssignee(ctx, tenantId, conversationId, agentId); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
