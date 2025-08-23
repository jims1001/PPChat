package model

import (
	"time"
)

// Status
const (
	GroupStatusNormal   int32 = 0
	GroupStatusBanned   int32 = 1
	GroupStatusDismiss  int32 = 2
	GroupStatusReadOnly int32 = 3
)

// GroupType
const (
	GroupTypeNormal  int32 = 1
	GroupTypeSuper   int32 = 2
	GroupTypeChannel int32 = 3
	GroupTypePrivate int32 = 4
)

// NeedVerification
const (
	VerifyNone       int32 = 0 // 直接加入
	VerifyAdmin      int32 = 1 // 管理员审批
	VerifyQnA        int32 = 2 // 回答入群问题
	VerifyInviteOnly int32 = 3 // 仅邀请链接
)

// QAItem 表示入群验证的问题和答案（可用于答题验证模式）。
type QAItem struct {
	Question string   `bson:"question"` // 问题
	Answers  []string `bson:"answers"`  // 正确答案列表（可多选）
}

// Group 表示群/频道/超级群的元数据。
// 仅存“群本身”的配置与状态；与消息、成员、会话分离。
type Group struct {
	TenantID  string `bson:"tenant_id"`  // PK
	GroupID   string `bson:"group_id"`   // 群ID（全局唯一），作为分片/路由键合适
	GroupName string `bson:"group_name"` // 群名称（可改名需计入版本/更新时间）
	FaceURL   string `bson:"face_url"`   // 群头像

	// 公开信息
	Introduction           string    `bson:"introduction"`             // 群简介/描述（可多语言外置，存默认语言）
	Notification           string    `bson:"notification"`             // 群公告（富文本可存外部对象；此处存纯文本或摘要）
	NotificationUpdateTime time.Time `bson:"notification_update_time"` // 公告更新时间
	NotificationUserID     string    `bson:"notification_user_id"`     // 最后更新公告的人

	// 基本元信息
	CreatorUserID string    `bson:"creator_user_id"` // 创建者（可在转让时变更）
	CreateTime    time.Time `bson:"create_time"`     // 创建时间
	UpdateTime    time.Time `bson:"update_time"`     // 最后一次任何字段的更新时间（写路径统一维护）
	Status        int32     `bson:"status"`          // 群状态：0=正常, 1=封禁, 2=解散(逻辑删除), 3=冻结只读
	GroupType     int32     `bson:"group_type"`      // 群类型：1=普通群, 2=超级群/大群, 3=频道(单向), 4=私密群

	// 入群/隐私设置
	NeedVerification int32  `bson:"need_verification"` // 0=无需验证,1=管理员审批,2=回答问题,3=仅邀请链接
	IsDiscoverable   bool   `bson:"is_discoverable"`   // 是否可被搜索/发现（false=仅凭邀请/链接进入）
	IsJoinByLink     bool   `bson:"is_join_by_link"`   // 是否允许邀请链接加入
	InviteLinkHash   string `bson:"invite_link_hash"`  // 邀请链接标识（旋转无效旧链接时更新）

	VerificationQAs  []QAItem `bson:"verification_qas"`  // 入群验证问题（可多个）
	VerificationMode int32    `bson:"verification_mode"` // 验证模式: 0=仅留言,1=选择题,2=问答题,3=混合
	VerificationTips string   `bson:"verification_tips"` // 验证提示信息（例如“请填写工号”）

	// 成员可见性/社交
	LookMemberInfo    int32 `bson:"look_member_info"`    // 0=仅成员可见资料, 1=所有人可见（列表可见/不可见）
	ApplyMemberFriend int32 `bson:"apply_member_friend"` // 0=不可互加, 1=允许成员相互加好友（产品策略）

	// 管理/风控（常见开关）
	MuteAll          bool  `bson:"mute_all"`           // 全员禁言（管理员/白名单例外）
	SlowModeInterval int32 `bson:"slow_mode_interval"` // 慢速模式：成员发言间隔(秒)，0=关闭
	RetentionDays    int32 `bson:"retention_days"`     // 消息留存天数（0=永久）；离线清理策略配合
	AllowAtAll       bool  `bson:"allow_at_all"`       // 是否允许 @全体 成员
	MediaMaxSizeMB   int32 `bson:"media_max_size_mb"`  // 上传媒体大小上限

	// 统计/只读缓存（写路径维护，供展示与排序）
	MemberCount int32 `bson:"member_count"` // 当前成员数（用于发现页/排序，异步一致可容忍轻微漂移）
	AdminCount  int32 `bson:"admin_count"`  // 管理员/版主数量

	// 扩展区
	Tags   []string `bson:"tags"`   // 群标签（兴趣/地域等）
	Region string   `bson:"region"` // 区域/语言标识（如 "CN", "US", "en-US"）
	Ex     string   `bson:"ex"`     // 预留JSON扩展（小量结构化配置可外置到 Settings 集合）

	// 兼容/迁移
	SchemaVersion int32      `bson:"schema_version"`       // 文档结构版本（灰度升级/后向兼容）
	DeletedAt     *time.Time `bson:"deleted_at,omitempty"` // 逻辑删除/解散时间（Status=2 时有效）
}
