package model

import "time"

type UserSession struct {
	// —— 基础标识 ——
	SessionID string `bson:"session_id" json:"session_id"`         // 会话ID（UUID/雪花）
	UserID    string `bson:"user_id" json:"user_id"`               // 用户ID（外键关联 User.user_id）
	TenantID  string `bson:"tenant_id,omitempty" json:"tenant_id"` // 多租户ID（可选）

	// —— 设备与环境 ——
	DeviceType string `bson:"device_type" json:"device_type"`       // 设备类型（web/ios/android/pc等）
	DeviceID   string `bson:"device_id,omitempty" json:"device_id"` // 设备唯一标识
	AppVersion string `bson:"app_version,omitempty" json:"app_version"`
	OS         string `bson:"os,omitempty" json:"os"` // 操作系统版本
	IP         string `bson:"ip" json:"ip"`           // 登录IP
	UserAgent  string `bson:"user_agent,omitempty" json:"user_agent"`

	AccessToken string `bson:"access_token,omitempty" json:"access_token"`
	// —— 时间与状态 ——
	LoginTime  time.Time  `bson:"login_time" json:"login_time"`   // 登录时间
	LastActive time.Time  `bson:"last_active" json:"last_active"` // 最后活跃时间
	ExpireTime time.Time  `bson:"expire_time" json:"expire_time"` // 过期时间（业务）
	ExpireAt   time.Time  `bson:"expire_at" json:"expire_at"`     // TTL索引用
	LogoutTime *time.Time `bson:"logout_time,omitempty" json:"logout_time"`
	Status     string     `bson:"status" json:"status"` // online/offline/kicked/expired
	Reason     string     `bson:"reason" json:"reason"` // 备注

	// —— 认证与安全 ——
	AccessTokenHash  string   `bson:"access_token_hash" json:"access_token_hash"`   // AccessToken 哈希
	RefreshTokenHash string   `bson:"refresh_token_hash" json:"refresh_token_hash"` // RefreshToken 哈希
	Scope            []string `bson:"scope,omitempty" json:"scope"`                 // 权限范围
	IsValid          bool     `bson:"is_valid" json:"is_valid"`                     // 是否有效
	RiskScore        int      `bson:"risk_score,omitempty" json:"risk_score"`       // 风险评分

	// —— 审计与扩展 ——
	Ex         string    `bson:"ex,omitempty" json:"ex"`         // 扩展字段(JSON)
	CreateTime time.Time `bson:"create_time" json:"create_time"` // 创建时间
	UpdateTime time.Time `bson:"update_time" json:"update_time"` // 更新时间
}
