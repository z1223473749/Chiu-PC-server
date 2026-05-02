package middleware

import (
	"ffmpegserver/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// releasePaths JWT 白名单路径，这些路径不需要认证
var releasePaths = map[string]bool{
	"/api/auth/login":   true,
	"/api/auth/refresh": true,
	"/api/health":       true,
}

// releasePrefixes 白名单路径前缀
var releasePrefixes = []string{
	"/swagger/",
	"/avatar/",
}

// JWTAuthMiddleware JWT 认证中间件
// 从 Authorization: Bearer <token> 中解析 JWT，并将 user_id 注入 gin.Context
// 白名单路径（login, refresh, swagger 等）无需认证
func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 白名单放行
		if releasePaths[path] {
			c.Next()
			return
		}
		for _, prefix := range releasePrefixes {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		// OPTIONS 预检请求放行
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		// 从请求头中提取 Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "缺少 Authorization 请求头"})
			return
		}

		// 检查 Bearer 前缀
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization 格式错误，应为: Bearer <token>"})
			return
		}

		tokenString := parts[1]

		// 解析 Token
		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token 无效或已过期"})
			return
		}

		// 校验 Token 类型必须是 access
		if claims.TokenType != "access" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token 类型错误，请使用 Access Token"})
			return
		}

		// 将用户信息注入上下文，供后续 handler 使用
		c.Set("user_id", claims.UserID)

		c.Next()
	}
}

// OptionalJWTAuthMiddleware 可选的 JWT 认证中间件
// 如果请求带有有效的 Token，则解析并注入 user_id；没有也能正常通过
func OptionalJWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := utils.ParseToken(parts[1])
		if err == nil && claims.TokenType == "access" {
			c.Set("user_id", claims.UserID)
		}

		c.Next()
	}
}

// GetUserIDFromContext 从上下文中获取当前登录的用户 ID
func GetUserIDFromContext(c *gin.Context) int32 {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	id, ok := userID.(int32)
	if !ok {
		return 0
	}
	return id
}
