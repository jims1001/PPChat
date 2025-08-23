package model

import "time"

// GroupBlack 表示群组黑名单记录。
// 一条记录对应“某个群组”里“某个被拉黑用户”的封禁状态。
// 通常在成员被移出群、拉黑时写入。
type GroupBlack struct {
	TenantID       string `bson:"tenant_id"`        // PK
	GroupID        string `bson:"group_id"`         // 群组ID（黑名单归属群）
	BlockUserID    string `bson:"block_user_id"`    // 被拉黑的用户ID
	OperatorUserID string `bson:"operator_user_id"` // 执行拉黑操作的人（群主/管理员ID）

	Reason     string    `bson:"reason"`                // 拉黑原因（文本说明，可选）
	CreateTime time.Time `bson:"create_time"`           // 拉黑时间
	ExpireTime time.Time `bson:"expire_time,omitempty"` // 过期时间；为空=永久拉黑

	Status        int32     `bson:"status"`          // 状态: 0=生效,1=解除,2=过期
	UnblockTime   time.Time `bson:"unblock_time"`    // 实际解禁时间
	UnblockUserID string    `bson:"unblock_user_id"` // 解禁操作人（管理员/系统）

	BanType int32  `bson:"ban_type"` // 0=拉黑禁止入群,1=禁言禁止发言,2=两者皆禁
	Scope   string `bson:"scope"`    // 生效范围: "group"(整个群), "channel"(子频道), "topic"(话题)

	Source    string `bson:"source"`     // 操作来源: manual/admin/bot/auto-detect
	DeviceID  string `bson:"device_id"`  // 用户设备标识（防止换号回群）
	IPAddress string `bson:"ip_address"` // 操作或违规时的IP（可选，审计）

	SyncSeq    int64     `bson:"sync_seq"`    // 同步序列号，用于多端同步黑名单更新
	UpdateTime time.Time `bson:"update_time"` // 最后一次修改时间（便于缓存更新）

	Ex string `bson:"ex"` // 预留扩展字段（JSON，可存客户端设备信息/来源等）
}
