package model

import "time"

type Tenant struct {
	TenantID      string       `bson:"tenant_id"` // PK
	Name          string       `bson:"name"`
	Plan          string       `bson:"plan"`           // basic/pro/enterprise
	Region        string       `bson:"region"`         // 数据驻留/合规区域
	Status        int32        `bson:"status"`         // 0=normal,1=suspended,2=closed
	RetentionDays int32        `bson:"retention_days"` // 消息留存
	Limits        TenantLimits `bson:"limits"`         // 并发/群成员上限/文件大小等
	Ex            string       `bson:"ex"`             // 扩展
	CreateTime    time.Time    `bson:"create_time"`
	UpdateTime    time.Time    `bson:"update_time"`
}
type TenantLimits struct {
	MaxUsers        int32 `bson:"max_users"`
	MaxGroups       int32 `bson:"max_groups"`
	MaxUploadMB     int32 `bson:"max_upload_mb"`
	MaxConnPerAgent int32 `bson:"max_conn_per_agent"`
}
