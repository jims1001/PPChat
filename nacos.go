package main

import (
	kafka2 "PProject/service/dispatcher/kafka"
	"PProject/service/kafka"
	"PProject/service/nacos"
	"fmt"
	"log"
	"net/http"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
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
	fmt.Println("nacos :\n", content)

	nacos.StartNacosWatcher("com.agent.agent-user", "DEFAULT_GROUP")
	nacos.StartServiceRegistry("agent.user-service")

	registry := nacos.NewRegistry("agent.gprc", "127.0.0.1", 50051)
	registry.AddRemoteService("UserService")
	registry.AddRemoteService("OrderService")

	registry.Watch()

	if err := kafka.Init([]string{"localhost:9092"}); err != nil {
		log.Fatalf("init producer faile: %v", err)
	}
	defer kafka.Close()

	err = kafka2.SendMessage("agent.user.login", "user_login", "user_id")
	if err != nil {
		println(err.Error())
	}

	fmt.Println("register success，running...")

	http.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, nacos.GetCurrentConfig())
	})

	fmt.Println("service running ： http://localhost:8080/config")
	http.ListenAndServe(":8080", nil)

}
