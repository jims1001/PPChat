package ids

import (
	"strings"

	"github.com/google/uuid"
)

type Kind string

const (
	KindAccount      Kind = "acct"
	KindPolicy       Kind = "pol"
	KindConversation Kind = "conv"
)

// Factory 统一ID工厂
// - UUID v7: 时间有序、全局唯一
// - 前缀：便于排查与区分实体类型
type Factory struct {
	// 可选：是否输出短格式（去掉 -）
	Compact bool
}

func New() *Factory {
	return &Factory{Compact: true}
}

// NewID 生成形如：acct_018f... 或 conv_018f...
func (f *Factory) NewID(kind Kind) string {
	u := uuid.Must(uuid.NewV7()).String()
	if f.Compact {
		u = strings.ReplaceAll(u, "-", "")
	}
	return string(kind) + "_" + u
}

// 便捷方法
func (f *Factory) NewAccountID() string      { return f.NewID(KindAccount) }
func (f *Factory) NewPolicyID() string       { return f.NewID(KindPolicy) }
func (f *Factory) NewConversationID() string { return f.NewID(KindConversation) }
