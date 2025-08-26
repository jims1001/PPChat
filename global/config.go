package global

import (
	"PProject/data/database/mgo/mongoutil"
	mid "PProject/middleware"
	mgoSrv "PProject/service/mgo"
	redis "PProject/service/storage/redis"
	ids "PProject/tools/ids"
	"context"
)

func ConfigIds() {
	ids.SetNodeID(100)
}

func GetJwtSecret() []byte {

	b := []byte("mN9b1f8zPq+W2xjX/45sKcVd0TfyoG+3Hp5Z8q9Rj1o=")
	return b
}

func ConfigRedis() {
	config := redis.Config{
		Addr: "127.0.0.1:7001", Password: "password", DB: 0,
	}
	err := redis.InitRedis(config)
	if err != nil {
		return
	}
}

func ConfigMgo() {

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cfg := &mongoutil.Config{
			Uri:         "mongodb://localhost:27017",
			Database:    "agentChat",
			MaxPoolSize: 20,
			Username:    "root",
			Password:    "example",
			MaxRetry:    3, // 这里不用了，StartAsync 里我们自己做了指数退避
		}

		// 1) 异步启动
		mgoSrv.StartAsync(ctx, cfg)
		err := mgoSrv.WaitReady(ctx, mgoSrv.Manager())
		if err != nil {
			return
		}
		select {
		case <-ctx.Done():
		}
	}()

}

func ConfigMiddleware() {
	mid.Config()
}
