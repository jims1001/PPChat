package model

import (
	"time"
)

// FriendRequest 表示一次好友申请的完整生命周期。
// 通常一条记录对应一方（FromUserID -> ToUserID）的单次申请。
// 在 MongoDB 中建议用 from_user_id + to_user_id + create_time 做联合索引。
type FriendRequest struct {
	TenantID   string `bson:"tenant_id"`    // PK
	FromUserID string `bson:"from_user_id"` // 发起方用户ID
	ToUserID   string `bson:"to_user_id"`   // 接收方用户ID

	HandleResult int32  `bson:"handle_result"` // 处理结果: 0=未处理, 1=同意, 2=拒绝, 3=忽略
	ReqMsg       string `bson:"req_msg"`       // 申请附带的留言信息

	CreateTime    time.Time `bson:"create_time"`     // 申请创建时间
	HandlerUserID string    `bson:"handler_user_id"` // 处理人ID（通常是 ToUserID；也可能是管理员）
	HandleMsg     string    `bson:"handle_msg"`      // 处理备注（如“抱歉不方便”）
	HandleTime    time.Time `bson:"handle_time"`     // 处理时间

	RequestID  string    `bson:"request_id"`  // 全局唯一请求ID（方便幂等处理）
	Status     int32     `bson:"status"`      // 状态(0=有效,1=已撤回,2=过期)
	ExpireTime time.Time `bson:"expire_time"` // 申请有效期（比如 7 天）

	AddSource  int32  `bson:"add_source"`  // 添加来源(1=手机号,2=群聊,3=二维码,4=系统推荐)
	AddChannel string `bson:"add_channel"` // 渠道(如 "ios_app", "web")

	ClientIP   string    `bson:"client_ip"`   // 发起请求的 IP
	DeviceID   string    `bson:"device_id"`   // 发起请求的设备ID
	UpdateTime time.Time `bson:"update_time"` // 最后一次更新时间

	IsRead     bool     `bson:"is_read"`     // 接收方是否已读申请
	RemarkName string   `bson:"remark_name"` // 期望对方的备注名
	Tags       []string `bson:"tags"`        // 申请时附带标签（如“同学”“同事”）

	Ex string `bson:"ex"` // 预留扩展字段（JSON，可存客户端设备信息等）
}
