package model

// ===== 常量 & 错误 =====
//
// singleDocMsgNum: 每个“消息文档分片”的容量上限，用于把同一会话消息分片存储（避免超大文档）。
// NewestList/OldestList: 作为游标或分页方向的语义常量。
const (
	singleDocMsgNum     = 100   // 单文档最多存放的消息条数（建议保留100~500之间）
	singleDocMsgNum5000 = 5000  // 兼容历史/大分片场景（不建议新用）
	MsgTableName        = "msg" // 集合名/表名
	OldestList          = 0
	NewestList          = -1
)

// ErrMsgListNotExist: 指定用户在 MongoDB 中没有消息列表（可用于首次加载的兜底）。
// var ErrMsgListNotExist = errs.New("user does not have messages in MongoDB")

// ===== 存储结构 =====

// MsgDocModel 表示“某个会话的一段消息分片文档”。
// 典型做法：按 conversation_id + doc_index 分片（此处省略 conversation_id 字段，建议新增，见扩展版）。
type MsgDocModel struct {
	DocID string          `bson:"doc_id"` // 分片文档ID（建议规则：<convID>:<docIndex>，便于路由/定位）
	Msg   []*MsgInfoModel `bson:"msgs"`   // 固定长度或上限长度的消息数组（append 到尾部）
}

// RevokeModel 表示撤回操作的元信息。
type RevokeModel struct {
	Role     int32  `bson:"role"`     // 操作人角色（0=普通成员,1=管理员,2=系统）
	UserID   string `bson:"user_id"`  // 操作人ID
	Nickname string `bson:"nickname"` // 操作人昵称（快照）
	Time     int64  `bson:"time"`     // 撤回时间(Unix ms)
}

// OfflinePushModel 作为离线推送的配置快照。
type OfflinePushModel struct {
	Title         string `bson:"title"`
	Desc          string `bson:"desc"`
	Ex            string `bson:"ex"`              // 扩展kv(JSON)
	IOSPushSound  string `bson:"ios_push_sound"`  // iOS自定义声音
	IOSBadgeCount bool   `bson:"ios_badge_count"` // 是否增加角标
}

// MsgDataModel 是一条消息的主干数据（消息本体）。
type MsgDataModel struct {
	TenantID string `bson:"tenant_id"` // PK
	// 路由/标识
	SendID           string `bson:"send_id"`            // 发送者ID
	RecvID           string `bson:"recv_id"`            // 单聊对端ID（群聊为空）
	GroupID          string `bson:"group_id"`           // 群聊ID（单聊为空）
	ClientMsgID      string `bson:"client_msg_id"`      // 客户端生成的幂等ID
	ServerMsgID      string `bson:"server_msg_id"`      // 服务端分配的全局/会话内消息ID
	SenderPlatformID int32  `bson:"sender_platform_id"` // 端来源（iOS/Android/Web等）
	SenderNickname   string `bson:"sender_nickname"`    // 发送者昵称（快照）
	SenderFaceURL    string `bson:"sender_face_url"`    // 发送者头像（快照）

	// 类型/内容
	SessionType int32  `bson:"session_type"` // 1=单聊,2=群聊,3=系统 4 是客服系统
	MsgFrom     int32  `bson:"msg_from"`     // 0=用户,1=系统/机器人
	ContentType int32  `bson:"content_type"` // 1=文本,2=图片,3=语音...（业务枚举）
	Content     string `bson:"content"`      // 内容（小体量直接存字符串；大体量建议对象存储）

	// 序号/时间
	Seq        int64 `bson:"seq"`         // 会话内自增序列（用于顺序/补偿拉取）
	SendTime   int64 `bson:"send_time"`   // 发送时间(Unix ms)
	CreateTime int64 `bson:"create_time"` // 创建时间(Unix ms)

	// 状态
	Status      int32             `bson:"status"`  // 0=正常,1=撤回,2=删除,3=折叠...
	IsRead      bool              `bson:"is_read"` // （可废弃，建议放用户视角的会话光标统计未读）
	Options     map[string]bool   `bson:"options"` // 杂项开关：已回执/阅后即焚/禁止转发等
	OfflinePush *OfflinePushModel `bson:"offline_push"`

	// @/扩展
	AtUserIDList []string `bson:"at_user_id_list"` // 被@的用户列表
	AttachedInfo string   `bson:"attached_info"`   // 附加信息(JSON)：引用、转发链、表情反应等
	Ex           string   `bson:"ex"`              // 预留扩展(JSON)
}

// MsgInfoModel 是“消息 + 操作痕迹”的包裹层，便于在同一数组位存放撤回/删除信息。
type MsgInfoModel struct {
	Msg     *MsgDataModel `bson:"msg"`      // 消息主体；撤回后可置nil，仅保留 Revoke
	Revoke  *RevokeModel  `bson:"revoke"`   // 撤回信息（若撤回）
	DelList []string      `bson:"del_list"` // 逻辑删除的用户ID集合（对这些用户不可见）
	IsRead  bool          `bson:"is_read"`  // （可选：历史遗留字段；建议转移到用户会话光标）
}

// 统计类
type UserCount struct {
	UserID string `bson:"user_id"`
	Count  int64  `bson:"count"`
}
type GroupCount struct {
	GroupID string `bson:"group_id"`
	Count   int64  `bson:"count"`
}

// ===== 方法 =====

func (*MsgDocModel) TableName() string { return MsgTableName }

// 单分片上限（100）
func (*MsgDocModel) GetSingleGocMsgNum() int64 { return singleDocMsgNum }

// 兼容历史（5000）
func (*MsgDocModel) GetSingleGocMsgNum5000() int64 { return singleDocMsgNum5000 }

// IsFull 判断当前分片是否已满。
// 原实现通过“最后一个元素的 msg 是否非空”判断，容易误判；建议直接判断长度到达上限。
func (m *MsgDocModel) IsFull() bool {
	return int64(len(m.Msg)) >= singleDocMsgNum
}

// GetDocIndex 根据会话内 Seq 计算分片下标（从0开始）。
// seq 从1起步，则 (seq-1)/singleDocMsgNum 为分片序号。
func (*MsgDocModel) GetDocIndex(seq int64) int64 {
	if seq <= 0 {
		return 0
	}
	return (seq - 1) / singleDocMsgNum
}
