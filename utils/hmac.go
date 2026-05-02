package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// hmacSecret HMAC 签名密钥
const hmacSecret = "Chiu-PC_2026_HMAC_Secret_Key"

// SignBody 对 body 进行 HMAC-SHA256 签名
func SignBody(body []byte) string {
	mac := hmac.New(sha256.New, []byte(hmacSecret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature 验证 body 的 HMAC 签名是否正确
func VerifySignature(body []byte, signature string) bool {
	expectedMAC := SignBody(body)
	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}

// ValidateHMAC 校验 HMAC 签名（兼容多种场景）
func ValidateHMAC(body []byte, signature string) bool {
	if signature == "" {
		return false
	}
	return VerifySignature(body, signature)
}
