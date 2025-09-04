package msgflow

import "context"

// 消息持久化模型
type Message struct {
	TenantID    string
	ConvID      string
	SenderID    string
	ClientMsgID string
	ServerMsgID string
	Seq         int64
	PayloadHash string
	Body        []byte
	CreatedAtMS int64
}

type MessageMeta struct {
	ServerMsgID string
	Seq         int64
	CreatedAtMS int64
}

// DB 抽象：生产实现 Mongo/MySQL；此处有内存实现（db_mem.go）
type DB interface {
	EnsureConversation(ctx context.Context, tenant, convID string) error
	QueryMaxSeq(ctx context.Context, tenant, convID string) (int64, error)

	InsertMessage(ctx context.Context, m *Message) error
	FindByClientID(ctx context.Context, tenant, sender, clientMsgID string) (*MessageMeta, error)
	FindBySeq(ctx context.Context, tenant, convID string, seq int64) (*MessageMeta, error)
	FindByServerID(ctx context.Context, serverMsgID string) (*MessageMeta, error)

	IsUniqueClientIDErr(err error) bool
	IsUniqueSeqErr(err error) bool
	IsUniqueServerIDErr(err error) bool
	IsTransientErr(err error) bool
}
