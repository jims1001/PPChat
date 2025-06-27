package config

import (
	"fmt"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"time"
)

func StartServiceRegistry(serviceName string) {
	go func() {
		client := InitNacosNamingClient()
		_, err := client.RegisterInstance(vo.RegisterInstanceParam{
			Ip:          "127.0.0.1",
			Port:        8080,
			ServiceName: serviceName,
			GroupName:   "DEFAULT_GROUP",
			Ephemeral:   true,
			Metadata: map[string]string{
				"protocol": "grpc",
				"version":  "v1",
				"auth":     "required",
			},
		})
		if err != nil {
			fmt.Println("register fail:", err)
			return
		}
		fmt.Println("register success")

		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			instances, err := client.SelectAllInstances(vo.SelectAllInstancesParam{
				ServiceName: "serviceName",
				GroupName:   "DEFAULT_GROUP",
			})
			if err != nil {
				fmt.Println("search fail:", err)
			} else {
				fmt.Printf("service list: %+v\n", instances)
			}
		}
	}()
}
