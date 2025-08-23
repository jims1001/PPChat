package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

// Friend 表示用户好友关系（单向存储，一般为双向各存一条记录）
// 在 MongoDB 中，通常以 owner_user_id + friend_user_id 作为唯一索引。
type Friend struct {
	TenantID     string             `bson:"tenant_id"`      // PK
	ID           primitive.ObjectID `bson:"_id"`            // MongoDB 主键
	OwnerUserID  string             `bson:"owner_user_id"`  // 拥有者用户ID（谁的好友列表）
	FriendUserID string             `bson:"friend_user_id"` // 好友用户ID（对方）

	Remark   string `bson:"remark"`    // 备注名（用户自己定义的昵称）
	IsPinned bool   `bson:"is_pinned"` // 是否置顶该好友会话

	IsBlocked bool  `bson:"is_blocked"` // 是否已拉黑该好友
	IsMuted   bool  `bson:"is_muted"`   // 是否屏蔽消息/免打扰
	Status    int32 `bson:"status"`     // 好友关系状态（0=待验证，1=已同意，2=已拒绝，3=已删除）

	Tags        []string  `bson:"tags"`         // 好友标签（比如“同事”“家人”）
	LastContact time.Time `bson:"last_contact"` // 最近联系时间（可做排序、推荐）

	UpdateTime time.Time `bson:"update_time"`           // 最后一次修改时间
	DeleteTime time.Time `bson:"delete_time,omitempty"` // 删除时间（逻辑删除保留记录）

	SyncSeq  int64  `bson:"sync_seq"`  // 同步序列号（方便多端同步）
	DeviceID string `bson:"device_id"` // 添加好友时使用的设备信息（可选，安全审计）

	CreateTime     time.Time `bson:"create_time"`      // 添加时间
	AddSource      int32     `bson:"add_source"`       // 添加来源（1=手机号，2=群组，3=二维码，4=搜索等）
	OperatorUserID string    `bson:"operator_user_id"` // 操作人（通常是自己；某些场景可能由管理员代操作）

	Ex string `bson:"ex"` // 预留扩展字段(JSON，可存标签、备注扩展等)
}
