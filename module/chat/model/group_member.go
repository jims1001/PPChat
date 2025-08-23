package model

import "time"

// GroupMember 表示群内的单个成员记录。
// 一条记录对应一个群 + 一个用户。
// 主要负责成员属性（昵称、头像）、角色权限、加入信息、禁言状态等。
type GroupMember struct {
	TenantID string `bson:"tenant_id"` // PK
	GroupID  string `bson:"group_id"`  // 群ID
	UserID   string `bson:"user_id"`   // 成员用户ID（唯一键: group_id+user_id）

	// —— 基本展示信息 ——
	Nickname string `bson:"nickname"` // 群内昵称（可与全局昵称不同）
	FaceURL  string `bson:"face_url"` // 群内头像（通常沿用用户头像，可覆盖）

	// —— 权限/角色 ——
	RoleLevel      int32 `bson:"role_level"`      // 角色等级: 0=普通成员,1=管理员,2=群主,3=子频道主持人...
	IsOwner        bool  `bson:"is_owner"`        // 是否为群主（可冗余 RoleLevel=2）
	IsAdmin        bool  `bson:"is_admin"`        // 是否为管理员（可冗余 RoleLevel=1）
	PermissionMask int64 `bson:"permission_mask"` // 可选: 二进制/位掩码表示细粒度权限（发公告、踢人、@全体）

	// —— 加入来源信息 ——
	JoinTime      time.Time `bson:"join_time"`       // 入群时间
	JoinSource    int32     `bson:"join_source"`     // 入群来源: 1=邀请链接,2=搜索申请,3=扫码,4=管理员拉入...
	InviterUserID string    `bson:"inviter_user_id"` // 邀请人ID（如果是被拉入）

	// —— 操作人/审计 ——
	OperatorUserID string `bson:"operator_user_id"` // 最后操作该成员数据的人（踢人/设管理员）

	// —— 风控状态 ——
	MuteEndTime time.Time `bson:"mute_end_time"` // 禁言截止时间（0或空=未禁言）
	IsBanned    bool      `bson:"is_banned"`     // 是否被群永久封禁（区别于临时禁言）
	BanReason   string    `bson:"ban_reason"`    // 封禁原因

	// —— 展示/偏好 ——
	DisplayOrder int32  `bson:"display_order"` // 用于排序（例如“群主优先、管理员靠前”）
	Remark       string `bson:"remark"`        // 成员备注（群主/管理员可维护）

	// —— 同步/扩展 ——
	UpdateTime time.Time `bson:"update_time"` // 最后更新时间
	Ex         string    `bson:"ex"`          // 扩展字段(JSON)，方便灰度/自定义数据

	Status   int32     `bson:"status"`    // 0=正常,1=已退出,2=被踢,3=禁入(黑名单)
	QuitTime time.Time `bson:"quit_time"` // 离开时间（退群/被踢）

	LastReadSeq int64     `bson:"last_read_seq"` // 已读消息序列（用于计算未读）
	LastActive  time.Time `bson:"last_active"`   // 最近发言时间
	MsgCount    int64     `bson:"msg_count"`     // 发言总数（可用于活跃度排行）

	Roles []string `bson:"roles"` // 多角色体系（Discord风格: Moderator, VIP…）
	Tags  []string `bson:"tags"`  // 群内标签/身份分组

	DeviceID  string `bson:"device_id"`  // 加入时的设备ID（可选）
	IPAddress string `bson:"ip_address"` // 加入/操作时的IP（风控）

}
