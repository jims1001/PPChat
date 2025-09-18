package config

import (
	registry "PProject/service/registry"
	"context"
	"log"
	"os"
	"time"
)

func RunService(srvName, serviceID string, port int, meta map[string]string) {

	// 后台跑 BootBlocking
	go func() {

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		consulAddr := os.Getenv("CONSUL_ADDR")
		if consulAddr == "" {
			consulAddr = "127.0.0.1:8500"
		}

		reg, err := registry.NewConsul(consulAddr)
		if err != nil {
			log.Fatalf("[manager] new consul error: %v", err)
		}

		// 初始化全局单例 Manager
		registry.InitDefault(reg, 20*time.Second)

		clear(meta)
		self := registry.Instance{
			Service:  srvName,
			ID:       serviceID,
			Address:  "127.0.0.1",
			Port:     port,
			Metadata: meta,
		}

		registry.Global().SetSelf(
			self,
			registry.RegisterOptions{
				HealthURL: "http://127.0.0.1:8080/health", // 或换成 GRPC 健康
			},
			srvName,
		)

		if err := registry.Global().BootBlocking(ctx, self, 5*time.Second, srvName); err != nil {
			log.Printf("[manager] boot error: %v", err)
		}
		// 主协程阻塞，直到收到退出信号

		for {
			time.Sleep(100 * time.Millisecond)
		}
	}()

}
