package model

import (
	"PProject/service/mgo"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

// SeqConversation 维护“某个会话消息流”的全局水位与保留范围。
// 侧重于：已提交的最大序号、可读的最小序号（历史清理后），以及压缩/迁移等场景的元信息。
// 注意：用户侧已读游标请放在 Conversation(用户视角) 表，不放这里。
type SeqConversation struct {
	TenantID       string `bson:"tenant_id"`       // PK
	ConversationID string `bson:"conversation_id"` // 会话ID（p2p:min:max / grp:<gid> / thread:<id>）
	MaxSeq         int64  `bson:"max_seq"`         // 当前已“提交可读”的最大消息序号（commit waterline）
	MinSeq         int64  `bson:"min_seq"`         // 当前仍保留的最小消息序号（历史清理后的下界；读范围=(MinSeq,MaxSeq]）

	// —— 可选：发号/压缩/迁移相关 ——
	IssuedSeq int64  `bson:"issued_seq,omitempty"` // 已“预分配/发号”的最大序号（>= MaxSeq；两阶段写时用于监控缺口）
	CompactWM int64  `bson:"compact_wm,omitempty"` // 压缩/归档安全水位（<= 可删除上界的安全检查点）
	Epoch     int32  `bson:"epoch,omitempty"`      // 迁移/灾备切换时的纪元号；(epoch,msgflow) 共同保证单调性
	ShardKey  string `bson:"shard_key,omitempty"`  // 路由/分片键（多分片部署时用于定位）

	// —— 仅元数据时间 ——
	CreateTime time.Time `bson:"create_time"` // 记录创建时间
	UpdateTime time.Time `bson:"update_time"` // 最近一次水位/元数据变更时间

	// —— 预留扩展 ——
	Ex string `bson:"ex,omitempty"` // JSON 扩展（非常见字段灰度）
}

func (sess *SeqConversation) GetTableName() string {
	return "seq_conversation"
}

func (sess *SeqConversation) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}
