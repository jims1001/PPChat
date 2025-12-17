package dto

type AddCollaboratorReq struct {
	AgentID string `json:"agent_id" binding:"required"`
	Role    string `json:"role,omitempty"` // collaborator|observer，默认 collaborator
}

type CollaboratorResp struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenant_id"`
	ConversationID string `json:"conversation_id"`
	AgentID        string `json:"agent_id"`
	Role           string `json:"role"`
	Status         string `json:"status"` // active|inactive
	JoinedAt       string `json:"joined_at"`
	LeftAt         string `json:"left_at,omitempty"`
}
