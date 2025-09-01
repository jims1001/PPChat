package kafka

// RegisterDefaultHandlers 给所有大 Topic 注册同一处理逻辑（你也可以按需分别注册）
func RegisterDefaultHandlers(topics []string, handler MessageHandler) {
	for _, t := range topics {
		RegisterHandler(t, handler)
	}
}
