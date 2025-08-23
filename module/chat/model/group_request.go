package model

import (
	"time"
)

// GroupRequest 表示一次入群申请。
// 一条记录对应一个用户对某个群的申请流程。
// 管理员/群主可以审批（同意/拒绝），也可以过期自动失效。
type GroupRequest struct {
	TenantID string `bson:"tenant_id"` // PK
	UserID   string `bson:"user_id"`   // 申请人用户ID
	GroupID  string `bson:"group_id"`  // 申请加入的群ID

	HandleResult int32  `bson:"handle_result"` // 处理结果: 0=待处理,1=同意,2=拒绝,3=忽略,4=过期
	ReqMsg       string `bson:"req_msg"`       // 申请附带留言（如“我是xx同学”）
	HandledMsg   string `bson:"handled_msg"`   // 审批回复/拒绝原因

	ReqTime      time.Time `bson:"req_time"`       // 申请时间
	HandleUserID string    `bson:"handle_user_id"` // 审批人（群主/管理员ID）
	HandledTime  time.Time `bson:"handled_time"`   // 审批时间

	JoinSource    int32  `bson:"join_source"`     // 入群来源: 1=搜索群号,2=邀请链接,3=二维码,4=成员邀请
	InviterUserID string `bson:"inviter_user_id"` // 邀请人（如果是被邀请的场景）

	RequestID  string    `bson:"request_id"`  // 全局唯一请求ID（便于幂等处理）
	Status     int32     `bson:"status"`      // 0=有效,1=撤回,2=过期
	ExpireTime time.Time `bson:"expire_time"` // 申请有效期（例如 7 天）

	VerifyAnswers []string `bson:"verify_answers"` // 入群问题回答（对应群设置的问题）
	VerifyPassed  bool     `bson:"verify_passed"`  // 是否通过自动题目验证

	ClientIP   string    `bson:"client_ip"`   // 申请时的 IP 地址
	DeviceID   string    `bson:"device_id"`   // 申请设备ID
	UpdateTime time.Time `bson:"update_time"` // 最后更新时间

	IsRead bool     `bson:"is_read"` // 管理员是否已读此请求
	Tags   []string `bson:"tags"`    // 申请标签（如“同学”“同事”）

	Ex string `bson:"ex"` // 扩展字段(JSON)，可存自定义信息

}
