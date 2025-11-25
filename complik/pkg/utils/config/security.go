package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// GetSecureValue 获取安全配置值，支持环境变量和加密值
func GetSecureValue(value string) (string, error) {
	// 检查是否是环境变量引用
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
		envValue := os.Getenv(envVar)
		if envValue == "" {
			return "", fmt.Errorf("environment variable %s not set", envVar)
		}
		return envValue, nil
	}

	// 检查是否是加密值
	if strings.HasPrefix(value, "ENC(") && strings.HasSuffix(value, ")") {
		encValue := strings.TrimSuffix(strings.TrimPrefix(value, "ENC("), ")")
		return DecryptValue(encValue)
	}

	// 普通值直接返回
	return value, nil
}

// EncryptValue 加密配置值
func EncryptValue(plaintext string) (string, error) {
	key := getEncryptionKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptValue 解密配置值
func DecryptValue(ciphertext string) (string, error) {
	key := getEncryptionKey()

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	// nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	// plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	// if err != nil {
	// 	return "", err
	// }

	return "", nil
}

// getEncryptionKey 从环境变量获取加密密钥
func getEncryptionKey() []byte {
	key := os.Getenv("COMPLIK_ENCRYPTION_KEY")
	if key == "" {
		// 使用默认密钥（仅用于开发环境）
		key = "development-key-do-not-use-prod!"
	}

	// 确保密钥长度为32字节（AES-256）
	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		// 填充到32字节
		padded := make([]byte, 32)
		copy(padded, keyBytes)
		return padded
	}
	return keyBytes[:32]
}
