package msgflow

import (
	"context"
)

type SeqManager interface {
	// 申请连续 seq（写入消息前）
	Alloc(ctx context.Context, tenant, conv string, need int64) (start int64, mill int64, err error)
	// 提交可读水位（消息写盘成功后推进）
	Commit(ctx context.Context, tenant, conv string, toSeq int64) error
	// 提升最小水位（清理历史后推进）
	TrimMin(ctx context.Context, tenant, conv string, newMin int64) error
	// 纠偏：把 issued 底线抬高（冷启动/Redis回退时）
	RaiseFloor(ctx context.Context, tenant, conv string, floor int64) error
}
