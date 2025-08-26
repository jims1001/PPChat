package storage

import (
	errors "PProject/tools/errs"
	"sync"
)

// 全局单例
var (
	manager     *OnlineStore
	managerOnce sync.Once
)

// InitManager 初始化全局 Manager，只能调用一次。
// 后续再次调用会返回同一个 Manager。
func InitManager(conf OnlineConfig) (*OnlineStore, error) {
	var err error
	managerOnce.Do(func() {

		m := newOnlineStore(conf)
		if err != nil {
			return
		}
		m.initExtraScripts()
		manager = m
	})
	if manager == nil {
		if err == nil {
			err = errors.New("manager initialization failed")
		}
		return nil, err
	}
	return manager, nil
}

// GetManager 获取全局 Manager，如果还没初始化会 panic。
func GetManager() *OnlineStore {
	if manager == nil {
		panic("Manager not initialized, call InitManager first")
	}
	return manager
}
