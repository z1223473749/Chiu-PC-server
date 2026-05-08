package config

// GlobalConfig 全局配置
var Config GlobalConfig

// MysqlConfig 数据库配置
type MysqlConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret           string `yaml:"secret"`             // JWT 签名密钥
	AccessExpireMin  int    `yaml:"access_expire_min"`  // Access Token 过期分钟
	RefreshExpireDay int    `yaml:"refresh_expire_day"` // Refresh Token 过期天数
	Issuer           string `yaml:"issuer"`             // 签发者
}

// LogConfig 日志配置
type LogConfig struct {
	LogFilePath string `yaml:"log_file_path"`
	LogFileName string `yaml:"log_file_name"`
	MaxSize     int    `yaml:"max_size"`
	MaxAge      int    `yaml:"max_age"`
	MaxBackups  int    `yaml:"max_backups"`
	Compress    bool   `yaml:"compress"`
}

// ApiDocConfig Swagger配置
type ApiDocConfig struct {
	Open bool `yaml:"open"`
}

// ServerConfig 服务配置
type ServerConfig struct {
	Port string `yaml:"port"`
}

// WsConfig WebSocket 配置
type Ws struct {
	SslCertificate    string `yaml:"SslCertificate"`
	SslCertificateKey string `yaml:"SslCertificateKey"`
	Port              int    `yaml:"port"`
}

// TaskConfig 任务配置
type TaskConfig struct {
	EncryptKey        string `yaml:"encrypt_key"`
	DefaultConcurrent int    `yaml:"default_concurrent"`
	MaxConcurrent     int    `yaml:"max_concurrent"`
}

// GlobalConfig 全局配置结构体
type GlobalConfig struct {
	ServerConfig `yaml:"server"`
	Ws           Ws         `yaml:"ws"`
	Task         TaskConfig `yaml:"task"`
	MysqlConfig  `yaml:"mysql"`
	RedisConfig  `yaml:"redis"`
	JWTConfig    `yaml:"jwt"`
	LogConfig    `yaml:"log"`
	ApiDocConfig `yaml:"apiDoc"`
}
