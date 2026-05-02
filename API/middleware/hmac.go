package middleware

import (
	"bytes"
	"ffmpegserver/utils"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// hmacReleasePaths HMAC 签名白名单路径
var hmacReleasePaths = map[string]bool{
	"/api/auth/login":   true,
	"/api/auth/refresh": true,
	"/api/auth/check":   true,
	"/api/health":       true,
}

// hmacReleasePrefixes HMAC 签名白名单路径前缀
var hmacReleasePrefixes = []string{
	"/swagger/",
	"/avatar/",
}

// HmacAuthMiddleware HMAC 签名校验中间件
// 客户端需要对 POST/PUT/DELETE 请求的 body 进行 HMAC-SHA256 签名，
// 并通过 X-Signature 请求头传递签名值
func HmacAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 白名单放行
		if hmacReleasePaths[path] {
			c.Next()
			return
		}
		for _, prefix := range hmacReleasePrefixes {
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

		// 仅校验 POST、PUT、DELETE 请求
		if c.Request.Method != http.MethodPost &&
			c.Request.Method != http.MethodPut &&
			c.Request.Method != http.MethodDelete {
			c.Next()
			return
		}

		// 读取原始 body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "读取请求体失败"})
			return
		}

		// 空 body 不校验签名
		if len(body) == 0 {
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
			c.Next()
			return
		}

		// 获取签名
		signature := c.GetHeader("X-Signature")
		if signature == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "缺少 X-Signature 请求头"})
			return
		}

		// 校验签名
		if !utils.ValidateHMAC(body, signature) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "签名校验失败"})
			return
		}

		// 重新填充 body（因为 ReadAll 会清空）
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}
