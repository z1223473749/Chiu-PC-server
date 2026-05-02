package redis

import (
	"context"
	"ffmpegserver/config"
	"fmt"
	"log"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

var (
	Rdb *goredis.Client
	ctx = context.Background()
)

// InitRedis 初始化 Redis 客户端
func InitRedis() {
	Rdb = goredis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Config.RedisConfig.Host, config.Config.RedisConfig.Port),
		Password: config.Config.RedisConfig.Password,
		DB:       0,
	})

	// 测试连接
	if err := Rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("[Redis] 连接失败: ", err)
	}

	fmt.Println("[Redis] 连接成功")
}

// RedisSet 写入缓存
func RedisSet(key string, value interface{}, expiration time.Duration) error {
	return Rdb.Set(ctx, key, value, expiration).Err()
}

// RedisGet 读取缓存
func RedisGet(key string) (string, error) {
	return Rdb.Get(ctx, key).Result()
}

// RedisDel 删除缓存
func RedisDel(key string) error {
	return Rdb.Del(ctx, key).Err()
}

// RedisExists 检查键是否存在
func RedisExists(key string) (bool, error) {
	n, err := Rdb.Exists(ctx, key).Result()
	return n > 0, err
}
