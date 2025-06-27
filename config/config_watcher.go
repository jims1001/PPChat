package config

import (
	"fmt"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

var (
	CurrentConfig string
	configMu      sync.RWMutex
)

// StartNacosWatcher 启动一个 goroutine 监听配置
func StartNacosWatcher(dataId, group string) {
	go func() {
		serverConfigs := []constant.ServerConfig{
			*constant.NewServerConfig("127.0.0.1", 8848),
		}

		clientConfig := *constant.NewClientConfig(
			constant.WithTimeoutMs(5000),
			constant.WithNamespaceId(""),
			constant.WithNotLoadCacheAtStart(true),
			constant.WithLogLevel("debug"),
		)

		configClient, err := clients.NewConfigClient(vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		})
		if err != nil {
			panic(err)
		}

		// 第一次读取
		content, err := configClient.GetConfig(vo.ConfigParam{
			DataId: dataId,
			Group:  group,
		})
		if err != nil {
			fmt.Println("获取配置失败:", err)
			return
		}
		updateConfig(content)

		// 开始监听
		err = configClient.ListenConfig(vo.ConfigParam{
			DataId: dataId,
			Group:  group,
			OnChange: func(namespace, group, dataId, data string) {
				fmt.Println("监听到配置变化:")
				fmt.Println("config", data)
				updateConfig(data)
			},
		})
		if err != nil {
			fmt.Println("监听配置失败:", err)
			return
		}

		select {} // 阻止 goroutine 退出
	}()
}

func updateConfig(data string) {
	configMu.Lock()
	defer configMu.Unlock()
	CurrentConfig = data
}

func GetCurrentConfig() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return CurrentConfig
}
