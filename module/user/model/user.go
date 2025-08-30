package model

import (
	"time"

	mgo "PProject/service/mgo"

	"go.mongodb.org/mongo-driver/mongo"
)

// GlobalRecvMsgOpt
const (
	RecvNotifyAll int32 = 0 // 接收并提醒
	RecvSilent    int32 = 1 // 接收不提醒
	RecvBlocked   int32 = 2 // 屏蔽
)

// AppMangerLevel
const (
	RoleUser  int32 = 0
	RoleAdmin int32 = 1
	RoleOwner int32 = 2
)

// Status
const (
	UserNormal   int32 = 0
	UserBanned   int32 = 1
	UserClosed   int32 = 2
	UserReadOnly int32 = 3
)

// User 表示系统中的用户（客户/座席/管理员/机器人等）。
// 仅放“用户主档”关键信息；偏好/安全/设备等建议拆表。
type User struct {
	// —— 基础标识 ——
	UserID   string `bson:"user_id" json:"UserID"`               // 全局唯一、不可变的用户ID（主键）
	TenantID string `bson:"tenant_id,omitempty" json:"TenantID"` // 多租户场景（客服系统强烈建议加）
	Nickname string `bson:"nickname" json:"Nickname"`            // 显示名
	FaceURL  string `bson:"face_url" json:"FaceURL"`             // 头像URL
	Bio      string `bson:"bio,omitempty" json:"Bio"`            // 个性签名/简介（可选）

	// —— 账号类型与状态 ——
	AccountType    int32      `bson:"account_type,omitempty" json:"AccountType"` // 0=普通用户,1=座席,2=机器人,3=系统账号
	AppMangerLevel int32      `bson:"app_manger_level" json:"AppMangerLevel"`    // 应用级权限：0=普通,1=管理员,2=超管
	Status         int32      `bson:"status,omitempty" json:"Status"`            // 0=正常,1=禁用,2=注销,3=冻结只读
	IsDeleted      bool       `bson:"is_deleted,omitempty" json:"IsDeleted"`     // 逻辑删除标记
	DeletedAt      *time.Time `bson:"deleted_at,omitempty" json:"DeletedAt"`     // 逻辑删除时间

	// —— 全局消息偏好/通知 ——
	GlobalRecvMsgOpt int32      `bson:"global_recv_msg_opt" json:"GlobalRecvMsgOpt"` // 0=接收并提醒,1=接收不提醒,2=屏蔽
	MuteUntil        *time.Time `bson:"mute_until,omitempty" json:"MuteUntil"`       // 全局免打扰至某时（可空）
	Language         string     `bson:"language,omitempty" json:"Language"`          // 首选语言（如 "zh-CN"）
	Timezone         string     `bson:"timezone,omitempty" json:"Timezone"`          // 时区（如 "Asia/Shanghai"）

	// —— 联系方式（可选：若有外部IAM可不放此处）——
	Phone         string `bson:"phone,omitempty" json:"Phone"`
	Email         string `bson:"email,omitempty" json:"Email"`
	PhoneVerified bool   `bson:"phone_verified,omitempty" json:"PhoneVerified"`
	EmailVerified bool   `bson:"email_verified,omitempty" json:"EmailVerified"`

	// —— 安全与审计（概要，详细建议拆表）——
	TwoFAEnabled  bool       `bson:"two_fa_enabled,omitempty"` // 是否开启二次验证
	LastLoginIP   string     `bson:"last_login_ip,omitempty"`
	LastLoginTime *time.Time `bson:"last_login_time,omitempty"`

	// —— 活跃度/在线状态（展示用，允许轻微不一致）——
	Presence   string    `bson:"presence,omitempty"` // online/away/dnd/offline
	LastActive time.Time `bson:"last_active,omitempty"`

	// —— 时间与扩展 ——
	CreateTime time.Time `bson:"create_time"` // 创建时间
	UpdateTime time.Time `bson:"update_time"` // 最后更新时间（任何字段变化都刷新）
	Ex         string    `bson:"ex"`          // 预留扩展(JSON)
}

func (u *User) GetNickname() string {
	return u.Nickname
}

func (u *User) GetFaceURL() string {
	return u.FaceURL
}

func (u *User) GetUserID() string {
	return u.UserID
}

func (u *User) GetEx() string {
	return u.Ex
}

func (u *User) GetTableName() string {
	return "user"
}

func (u *User) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(u.GetTableName())
}
