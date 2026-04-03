package main

import (
	"fmt"
	"log"
	"simple_ai/common/mysql"
	"simple_ai/common/rabbitmq"
	"simple_ai/common/redis"
	"simple_ai/config"
	"simple_ai/router"
)

func StartServer(addr string, port int) error {
	r := router.InitRouter()
	//服务器静态资源路径映射关系，这里目前不需要
	// r.Static(config.GetConfig().HttpFilePath, config.GetConfig().MusicFilePath)
	return r.Run(fmt.Sprintf("%s:%d", addr, port))
}

func main() {
	conf := config.GetConfig()
	host := conf.MainConfig.Host
	port := conf.MainConfig.Port
	//初始化mysql
	if err := mysql.InitMysql(); err != nil {
		log.Println("InitMysql error , " + err.Error())
		return
	}

	//初始化redis
	redis.Init()
	log.Println("redis init success  ")
	rabbitmq.InitRabbitMQ()
	log.Println("rabbitmq init success  ")

	err := StartServer(host, port) // 启动 HTTP 服务
	if err != nil {
		panic(err)
	}
}
