package msgflow

import "github.com/google/uuid"

// ServerMsgID 生成器接口（可接 Snowflake/ULID）
type ServerIDGenerator interface{ New() string }

// 默认 UUID 实现（生产建议替换为 Snowflake/ULID）
type UUIDGen struct{}

func (UUIDGen) New() string { return uuid.NewString() }
