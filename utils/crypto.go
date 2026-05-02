package utils

import (
	"ffmpegserver/config"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ==================== bcrypt 密码加密 ====================

// HashPassword 对明文密码进行 bcrypt 加密
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("密码加密失败: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword 校验明文密码与 bcrypt 哈希是否匹配
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ==================== JWT Token ====================

// CustomClaims JWT 自定义声明
type CustomClaims struct {
	UserID    int32  `json:"user_id"`
	TokenType string `json:"token_type"` // "access" 或 "refresh"
	jwt.RegisteredClaims
}

// GenerateAccessToken 生成 Access Token（短期有效）
func GenerateAccessToken(userID int32) (string, error) {
	cfg := config.Config.JWTConfig
	claims := CustomClaims{
		UserID:    userID,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(cfg.AccessExpireMin) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    cfg.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.Secret))
	if err != nil {
		return "", fmt.Errorf("AccessToken 生成失败: %w", err)
	}
	return tokenString, nil
}

// GenerateRefreshToken 生成 Refresh Token（长期有效）
func GenerateRefreshToken(userID int32) (string, error) {
	cfg := config.Config.JWTConfig
	claims := CustomClaims{
		UserID:    userID,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(cfg.RefreshExpireDay) * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    cfg.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.Secret))
	if err != nil {
		return "", fmt.Errorf("RefreshToken 生成失败: %w", err)
	}
	return tokenString, nil
}

// ParseToken 解析 JWT Token，返回 Claims
func ParseToken(tokenString string) (*CustomClaims, error) {
	cfg := config.Config.JWTConfig

	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非预期的签名算法: %v", token.Header["alg"])
		}
		return []byte(cfg.Secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("Token 解析失败: %w", err)
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("Token 无效")
	}

	return claims, nil
}
