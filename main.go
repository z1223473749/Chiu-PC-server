package main

import (
	"ffmpegserver/API"
	"ffmpegserver/API/login"
	"ffmpegserver/config"
	"ffmpegserver/public/redis"
	"ffmpegserver/public/sql"
	"ffmpegserver/service/video_dedup"
	"ffmpegserver/service/ws"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	// 初始化配置
	if err := config.InitGlobalConfig(); err != nil {
		panic(err)
	}

	// 初始化数据库
	sql.InitDB()

	// 正式环境下自动迁移表结构
	if os.Getenv("ENV") == "prod" {
		sql.AutoMigrateDB()
	}
	//sql.AutoMigrateDB()
	// 初始化 Redis
	redis.InitRedis()

	// 初始化默认头像
	login.InitAvatars("public/avatar")

	// 启动 WebSocket Hub 和 WS 服务
	go ws.GlobalWsHub.Run()
	go ws.StartWsServer(config.Config.Ws.Port)

	// 启动任务调度器
	video_dedup.StartScheduler()

	// 启动 HTTP 服务
	port := config.Config.ServerConfig.Port
	if port == "" {
		port = "9902"
	}

	htp := &http.Server{
		Addr:         ":" + port,
		Handler:      API.Gin,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	fmt.Println("======================================")
	fmt.Println("  Chiu-PC-Server 启动成功")
	fmt.Println("  API 地址: http://localhost:" + port + "/api")
	fmt.Println("  Swagger: http://localhost:" + port + "/swagger/index.html")
	fmt.Println("======================================")

	if err := htp.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
