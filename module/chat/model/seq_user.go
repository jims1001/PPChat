package model

// SeqUser 维护“某个用户在某个会话里的消息序列光标”。
// 只记录与消息序号相关的水位，不混入通知/置顶等个性化配置。
// 约定：会话内消息序号 Seq 单调递增，从 1 开始。
type SeqUser struct {
	TenantID       string `bson:"tenant_id"`       // PK
	UserID         string `bson:"user_id"`         // 用户ID（分片/查询主键之一）
	ConversationID string `bson:"conversation_id"` // 会话ID（如 p2p:<u1>:<u2> / grp:<gid>）

	// —— 序列水位（均为会话内的消息 Seq） ——
	MinSeq  int64 `bson:"min_seq"`  // 用户本地/服务端允许读取的最小序号下界（历史清理或本地裁剪后的“保留下界”）
	MaxSeq  int64 `bson:"max_seq"`  // 用户端“已拉取/已知”的最大序号（通常等于或小于会话全局 MaxSeq）
	ReadSeq int64 `bson:"read_seq"` // 用户已读确认的最大序号（用于未读计算；单调不降）

	// —— 可选扩展（常用但不强制） ——
	DeliveredSeq int64 `bson:"delivered_seq,omitempty"` // 服务端确认“已投递到该用户至少一个设备”的最大序号
	MentionSeq   int64 `bson:"mention_seq,omitempty"`   // 已处理完 @ 提醒的最大序号（清角标用）
	DraftSeq     int64 `bson:"draft_seq,omitempty"`     // （可选）与草稿/引用锚点相关的序号（如定位光标）

	// —— 元信息 ——
	CreateTime int64  `bson:"create_time"`  // 创建时间(Unix ms)
	UpdateTime int64  `bson:"update_time"`  // 最近一次更新(Unix ms)
	Ex         string `bson:"ex,omitempty"` // 预留扩展(JSON)
}
