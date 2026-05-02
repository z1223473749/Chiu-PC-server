package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// InitGlobalConfig 初始化全局配置
// 根据 ENV 环境变量加载不同配置文件：
//   - dev  → config/config.dev.yaml
//   - prod → config/config.prod.yaml
//     其他   → config/config.yaml
func InitGlobalConfig() error {
	configPath := getConfigFilePath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := yaml.Unmarshal(data, &Config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	fmt.Printf("[配置] 已加载配置文件: %s (ENV=%s)\n", configPath, os.Getenv("ENV"))
	return nil
}

// getConfigFilePath 根据环境变量返回配置文件路径
func getConfigFilePath() string {
	env := os.Getenv("ENV")
	switch env {
	case "dev":
		return "./config/config.dev.yaml"
	case "prod":
		return "./config/config.prod.yaml"
	default:
		return "./config/config.yaml"
	}
}
