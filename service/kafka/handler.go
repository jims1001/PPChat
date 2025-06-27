package kafka

import (
	"fmt"
	"sync"
)

type MessageHandler func(topic string, key, value []byte) error

var (
	handlerMap = make(map[string]MessageHandler)
	mu         sync.RWMutex
)

func RegisterHandler(topic string, handler MessageHandler) {
	mu.Lock()
	defer mu.Unlock()
	handlerMap[topic] = handler
}

func GetHandler(topic string) (MessageHandler, error) {
	mu.RLock()
	defer mu.RUnlock()
	if h, ok := handlerMap[topic]; ok {
		return h, nil
	}
	return nil, fmt.Errorf("no handler registered for topic: %s", topic)
}
