package utils

import (
	"encoding/base64"
	"errors"
	"strings"
)

const TransportKey = "ChiuPC_Transport_XOR_2026"

// DecryptTransport 从 @XOR@<base64> 解密出明文
// 服务端用：客户端发来的 XOR 加密 → 解密得到完整 ffmpeg 命令
func DecryptTransport(cipherText string) ([]byte, error) {
	if !strings.HasPrefix(cipherText, "@XOR@") {
		return nil, errors.New("missing @XOR@ prefix")
	}

	b64 := cipherText[5:] // 去掉 @XOR@
	cipher, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}

	key := []byte(TransportKey)
	keyLen := len(key)

	plain := make([]byte, len(cipher))
	for i, b := range cipher {
		plain[i] = b ^ key[i%keyLen]
	}

	return plain, nil
}
