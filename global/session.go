package global

// UserSession 全局的接口请求 需要处理的session
type UserSession struct {
	SessionId string `json:"session_id"`
	TenantID  string `json:"tenant_id"` // 多租户场景（客服系统强烈建议加）
}
