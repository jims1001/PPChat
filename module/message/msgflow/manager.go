package msgflow

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type IMManager struct {
	db      DB
	rdb     redis.UniversalClient
	handler *Handler
}

func NewIMManager(db DB, rdb redis.UniversalClient, sidGen ServerIDGenerator) *IMManager {
	h := NewHandler(db, rdb, sidGen) // 组合 Index + Seq + SIDGen
	return &IMManager{
		db:      db,
		rdb:     rdb,
		handler: h,
	}
}

// 对外暴露的统一入口：发送一条消息
func (m *IMManager) SendMessage(ctx context.Context, tenant, convID, sender, clientMsgID string, body []byte) (*MessageMeta, error) {
	return m.handler.SaveMessage(ctx, tenant, convID, sender, clientMsgID, body)
}
