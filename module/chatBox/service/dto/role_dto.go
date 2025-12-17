package dto

type CreateRoleReq struct {
	AccountID string `json:"account_id" binding:"required"`
	Name      string `json:"name" binding:"required"`
	Code      string `json:"code" binding:"required"`
	Desc      string `json:"desc,omitempty"`
	BuiltIn   bool   `json:"built_in"`
}

type PatchRoleReq struct {
	Name   *string `json:"name,omitempty"`
	Desc   *string `json:"desc,omitempty"`
	Status *string `json:"status,omitempty"` // pending|active|disabled
}

type RoleResp struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Desc      string `json:"desc,omitempty"`
	BuiltIn   bool   `json:"built_in"`
	Status    string `json:"status"`
}
