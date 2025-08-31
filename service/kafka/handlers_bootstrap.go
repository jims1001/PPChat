package kafka

import "github.com/golang/glog"

// RegisterDefaultHandlers 给所有大 Topic 注册同一处理逻辑（你也可以按需分别注册）
func RegisterDefaultHandlers(topics []string) {
	h := func(topic string, key, value []byte) error {
		glog.Info("[HANDLER] topic=%s key=%s value=%s", topic, string(key), string(value))
		return nil
	}
	for _, t := range topics {
		RegisterHandler(t, h)
	}
}
