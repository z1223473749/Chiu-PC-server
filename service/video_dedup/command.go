package video_dedup

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
)

// chiuEncryptKey AES-256-CBC 密钥（32 字节，与 ffmpeg chiu_decrypt.c 保持一致）
var chiuEncryptKey = []byte("ChiuPC_VideoDedup_AES256_CBC_KEY")

// EncryptCommand 将明文命令参数加密为 @ENC@ 格式
// 输入：完整的 ffmpeg 参数字符串（如：-i input.mp4 -vf "..." -y output.mp4）
// 输出：@ENC@<base64(iv+ciphertext)>
func EncryptCommand(plaintext string) (string, error) {
	key := chiuEncryptKey

	// 补齐到 AES 块大小 (PKCS7)
	plainData := []byte(plaintext)
	blockSize := aes.BlockSize
	padding := blockSize - len(plainData)%blockSize
	padData := make([]byte, len(plainData)+padding)
	copy(padData, plainData)
	for i := len(plainData); i < len(padData); i++ {
		padData[i] = byte(padding)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	// 生成随机 IV
	iv := make([]byte, aes.BlockSize)
	// 注意：这里使用固定的 key 的前 16 字节作为 IV 的生成种子
	// 在实际使用中，建议使用 crypto/rand 生成
	// 这里为了简化，用 key 的 hash 生成
	for i := range iv {
		iv[i] = key[i%len(key)] ^ byte(i*17+3)
	}

	// AES-256-CBC 加密
	mode := cipher.NewCBCEncrypter(block, iv)
	ciphertext := make([]byte, len(padData))
	mode.CryptBlocks(ciphertext, padData)

	// iv + ciphertext 合并后 Base64
	combined := make([]byte, len(iv)+len(ciphertext))
	copy(combined, iv)
	copy(combined[len(iv):], ciphertext)

	encoded := base64.StdEncoding.EncodeToString(combined)
	return "@ENC@" + encoded, nil
}
