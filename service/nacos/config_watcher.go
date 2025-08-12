package nacos

import (
	"fmt"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

var (
	CurrentConfig string
	configMu      sync.RWMutex
)

func StartNacosWatcher(dataId, group string) {
	go func() {

		configClient := InitNacosConfigClient()

		content, err := configClient.GetConfig(vo.ConfigParam{
			DataId: dataId,
			Group:  group,
		})
		if err != nil {
			fmt.Println("get nacos fail:", err)
			return
		}
		updateConfig(content)

		err = configClient.ListenConfig(vo.ConfigParam{
			DataId: dataId,
			Group:  group,
			OnChange: func(namespace, group, dataId, data string) {
				fmt.Println("nacos change:")
				fmt.Println("nacos", data)
				updateConfig(data)
			},
		})
		if err != nil {
			fmt.Println("get nacos fail:", err)
			return
		}

		select {}
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
