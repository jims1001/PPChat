package dto

// --- Create Agent ---
type CreateAgentReq struct {
	AccountID string   `json:"account_id" binding:"required"`
	Name      string   `json:"name" binding:"required"`
	Role      string   `json:"role" binding:"required,oneof=agent manager admin"`
	Email     string   `json:"email" binding:"required,email"`
	Status    *int     `json:"status,omitempty"` // 可选：默认 1
	TeamIDs   []string `json:"team_ids,omitempty"`
	CreatedBy string   `json:"created_by,omitempty"` // 谁创建（审计用，可选）
}

// 建议创建后返回 agent + invite 的过期时间（token 生产环境一般不返回，只邮件发送）
type CreateAgentResp struct {
	Agent  AgentResp       `json:"agent"`
	Invite *InviteInfoResp `json:"invite,omitempty"`
}

type InviteInfoResp struct {
	Token     string `json:"token,omitempty"` // dev 环境可返回，prod 建议不返回
	ExpiresAt string `json:"expires_at"`
}

// --- List Agents ---
type ListAgentsQuery struct {
	Role   string `form:"role"`
	Status *int   `form:"status"`
	TeamID string `form:"team_id"`
	Email  string `form:"email"`
	Name   string `form:"name"`

	Page int64 `form:"page"` // 从 1 开始
	Size int64 `form:"size"` // 默认 20
}

// --- Patch Agent ---
type PatchAgentReq struct {
	Name    *string  `json:"name,omitempty"`
	Role    *string  `json:"role,omitempty" binding:"omitempty,oneof=agent manager admin"`
	Status  *int     `json:"status,omitempty"` // 1/0
	TeamIDs []string `json:"team_ids,omitempty"`
}

// --- Resend Invite ---
type ResendInviteReq struct {
	CreatedBy string `json:"created_by,omitempty"`
}

// --- Accept Invite ---
type AcceptInviteReq struct {
	Token string `json:"token" binding:"required"`
	// 可选：如果你要接受邀请时设置密码/2fa，这里加字段；但你当前 Agent 没 password 字段
	// Password  string `json:"password,omitempty"`
	// Enable2FA bool   `json:"enable_2fa,omitempty"`
}

type AgentResp struct {
	ID        string   `json:"id"`
	TenantID  string   `json:"tenant_id"`
	AccountID string   `json:"account_id"`
	Name      string   `json:"name"`
	Role      string   `json:"role"`
	Email     string   `json:"email"`
	Status    int      `json:"status"`
	TeamIDs   []string `json:"team_ids,omitempty"`
}
