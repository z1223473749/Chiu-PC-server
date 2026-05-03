package API

import (
	"ffmpegserver/API/dashboard"
	deviceapi "ffmpegserver/API/device"
	"ffmpegserver/API/login"
	"ffmpegserver/API/middleware"
	taskapi "ffmpegserver/API/video_dedup"
	"ffmpegserver/config"
	"fmt"

	_ "ffmpegserver/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var Gin *gin.Engine

func init() {
	// 根据配置设置 Gin 模式
	if !config.Config.ApiDocConfig.Open {
		gin.SetMode(gin.ReleaseMode)
	}

	Gin = gin.Default()

	// 注册中间件
	Gin.Use(middleware.CorsMiddleware())     // 跨域
	Gin.Use(middleware.HmacAuthMiddleware()) // HMAC 签名校验
	Gin.Use(middleware.JWTAuthMiddleware())  // JWT 认证

	// 注册路由
	apiGroup := Gin.Group("/api")
	{
		authGroup := apiGroup.Group("/auth")
		login.NewHandler().Register(authGroup)

		// 去重任务
		taskGroup := apiGroup.Group("/video-dedup")
		taskapi.NewHandler().Register(taskGroup)

		// 设备管理
		deviceGroup := apiGroup.Group("")
		deviceapi.NewHandler().Register(deviceGroup)

		// 仪表盘
		dashboardGroup := apiGroup.Group("")
		dashboard.NewHandler().Register(dashboardGroup)
	}

	// 健康检查
	Gin.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "Chiu-PC-Server",
		})
	})

	// 静态文件 - 头像
	Gin.Static("/avatar", "./public/avatar")

	// Swagger 文档
	if config.Config.ApiDocConfig.Open {
		Gin.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		fmt.Println("[Swagger] API 文档: http://localhost:" + config.Config.ServerConfig.Port + "/swagger/index.html")
	}
}
