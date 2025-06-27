package main

import (
	"fmt"
	"path"
	"reflect"
)

type RedisConfig struct {
	Addr string
}

func (RedisConfig) GetConfigFileName() string {
	return "redis.yaml"
}

type NotificationConfig struct {
	Endpoint string
}

func (NotificationConfig) GetConfigFileName() string {
	return "notification.yaml"
}

type AllConfig struct {
	Redis        RedisConfig
	Notification NotificationConfig
}

type cmds struct {
	config AllConfig
	conf   map[string]reflect.Value
}

func newCmds() *cmds {
	return &cmds{}
}

func (x *cmds) getTypePath(typ reflect.Type) string {
	return path.Join(typ.PkgPath(), typ.Name())
}

func (x *cmds) initAllConfig() {
	x.conf = make(map[string]reflect.Value)

	val := reflect.ValueOf(&x.config).Elem()
	num := val.NumField()
	for i := 0; i < num; i++ {
		field := val.Field(i)
		t := field.Type()
		pkt := x.getTypePath(t)
		x.conf[pkt] = field

		fmt.Printf("register types: %s => field value: %+v\n", pkt, field.Interface())
	}
}

func (x *cmds) getStringField(typePath, fieldName string) string {
	if v, ok := x.conf[typePath]; ok {
		return v.FieldByName(fieldName).String()
	}
	return ""
}

func main() {
	cmd := newCmds()

	cmd.config.Redis.Addr = "127.0.0.1:6379"
	cmd.config.Notification.Endpoint = "https://notify.example.com"

	cmd.initAllConfig()

	redisPath := cmd.getTypePath(reflect.TypeOf(RedisConfig{}))
	endpointPath := cmd.getTypePath(reflect.TypeOf(NotificationConfig{}))

	addr := cmd.getStringField(redisPath, "Addr")
	ep := cmd.getStringField(endpointPath, "Endpoint")

	fmt.Println("\n🚀 nacos filed：")
	fmt.Println("Redis Addr:", addr)
	fmt.Println("Notification Endpoint:", ep)

}
