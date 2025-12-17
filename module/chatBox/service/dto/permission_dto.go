package dto

type CreatePermissionReq struct {
	Code string `json:"code" binding:"required"`
	Name string `json:"name" binding:"required"`
	Desc string `json:"desc,omitempty"`
}

type PatchPermissionReq struct {
	Code *string `json:"code,omitempty"`
	Name *string `json:"name,omitempty"`
	Desc *string `json:"desc,omitempty"`
}

type ListPermissionQuery struct {
	Q    string `form:"q"`    // 模糊搜索 code/name
	Code string `form:"code"` // 精确
	Name string `form:"name"` // 精确或模糊自己选
	Page int64  `form:"page"` // 从1开始
	Size int64  `form:"size"` // 默认20
}

type PermissionResp struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
	Desc string `json:"desc,omitempty"`
}
