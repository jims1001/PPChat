package nacos

import (
	"log"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

var (
	onceConfig sync.Once
	configCli  config_client.IConfigClient

	onceNaming sync.Once
	namingCli  naming_client.INamingClient
)

func InitNacosConfigClient() config_client.IConfigClient {
	onceConfig.Do(func() {
		client, err := clients.NewConfigClient(vo.NacosClientParam{
			ClientConfig:  getClientConfig(),
			ServerConfigs: getServerConfig(),
		})
		if err != nil {
			log.Fatalf("create ConfigClient faild: %v", err)
		}
		configCli = client
	})
	return configCli
}

func InitNacosNamingClient() naming_client.INamingClient {
	onceNaming.Do(func() {
		client, err := clients.NewNamingClient(vo.NacosClientParam{
			ClientConfig:  getClientConfig(),
			ServerConfigs: getServerConfig(),
		})
		if err != nil {
			log.Fatalf("create NamingClient fail: %v", err)
		}
		namingCli = client
	})
	return namingCli
}

func getServerConfig() []constant.ServerConfig {
	return []constant.ServerConfig{
		*constant.NewServerConfig("127.0.0.1", 8848),
	}
}

func getClientConfig() *constant.ClientConfig {
	return constant.NewClientConfig(
		constant.WithNamespaceId("public"), //
		constant.WithTimeoutMs(5000),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogLevel("debug"),
		constant.WithCacheDir("nacos/cache"),
		constant.WithLogDir("nacos/log"),
		constant.WithUsername("user"),
		constant.WithPassword("123456"),
	)
}
