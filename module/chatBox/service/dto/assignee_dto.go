package dto

type SetAssigneeReq struct {
	AgentID   string   `json:"agent_id" binding:"required"`
	ContactID string   `json:"contact_id" binding:"required"`
	InboxID   string   `json:"inbox_id" binding:"required"`
	TeamID    string   `json:"team_id,omitempty"`
	Priority  string   `json:"priority,omitempty"` // none|low|normal|high|urgent
	TagIDs    []string `json:"tag_ids,omitempty"`
}

type AssigneeResp struct {
	ID             string   `json:"id"`
	TenantID       string   `json:"tenant_id"`
	ConversationID string   `json:"conversation_id"`
	AgentID        string   `json:"agent_id"`
	ContactID      string   `json:"contact_id"`
	InboxID        string   `json:"inbox_id"`
	TeamID         string   `json:"team_id,omitempty"`
	Priority       string   `json:"priority"`
	TagIDs         []string `json:"tag_ids,omitempty"`
	Status         string   `json:"status"` // active/inactive
	JoinedAt       string   `json:"joined_at"`
	LeftAt         string   `json:"left_at,omitempty"`
}
