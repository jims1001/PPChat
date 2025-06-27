package main

import (
	"awesomeProject1/config"
	"fmt"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"net/http"
)

func main() {

	serverConfigs := []constant.ServerConfig{
		*constant.NewServerConfig("127.0.0.1", 8848),
	}

	clientConfig := *constant.NewClientConfig(
		constant.WithNamespaceId("public"),
		constant.WithTimeoutMs(5000),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogDir("nacos/log"),
		constant.WithCacheDir("nacos/cache"),
		constant.WithLogLevel("debug"),
		constant.WithUsername("user"),
		constant.WithPassword("123456"),
	)

	configClient, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		panic(err)
	}

	content, err := configClient.GetConfig(vo.ConfigParam{
		DataId: "com.agent.agent-user",
		Group:  "DEFAULT_GROUP",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("config :\n", content)

	config.StartNacosWatcher("com.agent.agent-user", "DEFAULT_GROUP")

	http.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, config.GetCurrentConfig())
	})

	fmt.Println("service running ： http://localhost:8080/config")
	http.ListenAndServe(":8080", nil)

}
