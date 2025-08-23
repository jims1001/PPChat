package main

import (
	"PProject/data/database/mgo/mongoutil"
	mgoSrv "PProject/service/mgo"
	"context"
	"log"
	"time"
)

type User struct {
	UserID    string    `bson:"user_id"`
	Nickname  string    `bson:"nickname"`
	CreatedAt time.Time `bson:"created_at"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &mongoutil.Config{
		Uri:         "mongodb://localhost:27017",
		Database:    "chat",
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

	mgoSrv.GetDB().Collection("users")
	// 准备数据
	user := User{
		UserID:    "u1",
		Nickname:  "Alice",
		CreatedAt: time.Now(),
	}

	// 插入
	res, err := mgoSrv.GetDB().Collection("users").InsertOne(ctx, user)
	if err != nil {
		log.Fatalf("insert failed: %v", err)
	}

	log.Printf("inserted document with ID=%v", res.InsertedID)

	select {}

}
