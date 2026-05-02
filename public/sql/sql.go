package sql

import (
	"ffmpegserver/config"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var Gdb *gorm.DB

// InitDB 初始化数据库连接
func InitDB() {
	var err error
	dataSourceName := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.Config.MysqlConfig.Username,
		config.Config.MysqlConfig.Password,
		config.Config.MysqlConfig.Host,
		config.Config.MysqlConfig.Port,
		config.Config.MysqlConfig.Database,
	)

	Gdb, err = gorm.Open(mysql.New(mysql.Config{
		DSN:                      dataSourceName,
		DefaultStringSize:        256,
		DisableDatetimePrecision: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("[数据库] 连接失败: ", err)
	}

	fmt.Println("[数据库] MySQL 连接成功")
}
