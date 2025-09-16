package kafka

import (
	"fmt"
	"reflect"
	"sync"
)

type MessageHandler func(topic string, key, value []byte) error
type ProducerHandler func(topic, key string, value []byte) error

var (
	handlerMap = make(map[string]MessageHandler)
	mu         sync.RWMutex
)

func RegisterHandler(topic string, handler MessageHandler) (ok bool, duplicated bool) {
	if handler == nil {
		return false, false
	}
	mu.Lock()
	defer mu.Unlock()

	if old, exists := handlerMap[topic]; exists {
		oldPtr := reflect.ValueOf(old).Pointer()
		newPtr := reflect.ValueOf(handler).Pointer()

		if oldPtr == newPtr {
			// 同函数重复注册 → 幂等
			// logger.Warn("duplicate same handler", zap.String("topic", topic))
			return true, true
		}
		// 不同函数争夺同一 topic → 拒绝覆盖
		// logger.Warn("duplicate different handler (kept old)", zap.String("topic", topic))
		return false, true
	}

	handlerMap[topic] = handler
	return true, false
}

func GetHandler(topic string) (MessageHandler, error) {
	mu.RLock()
	defer mu.RUnlock()
	if h, ok := handlerMap[topic]; ok {
		return h, nil
	}
	return nil, fmt.Errorf("no handler registered for topic: %s", topic)
}
