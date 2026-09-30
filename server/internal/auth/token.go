package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/mrasong/tink/server/internal/store"
)

// GenerateToken 生成带 sk-tink- 前缀的随机安全 Secret Key
// 后接 64 位 0-9a-zA-Z 密码学安全随机字符串
func GenerateToken() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 64
	result := make([]byte, length)
	max := big.NewInt(int64(len(charset)))
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("crypto rand failed: %w", err)
		}
		result[i] = charset[num.Int64()]
	}
	return "sk-tink-" + string(result), nil
}

// AuthenticateKey 从 HTTP 提取的 rawToken 验证身份并返回 SecretKey
func AuthenticateKey(s *store.Store, rawToken string) (*store.SecretKey, error) {
	cleanToken := strings.TrimPrefix(rawToken, "Bearer ")
	cleanToken = strings.TrimSpace(cleanToken)
	if cleanToken == "" {
		return nil, fmt.Errorf("empty secret key")
	}

	hash := store.HashToken(cleanToken)
	key, err := s.GetKeyByHash(hash)
	if err != nil {
		return nil, err
	}

	// 校验 Secret Key 是否启用 (1: 启用, 0: 禁用)
	if key.Enabled != 1 {
		return nil, fmt.Errorf("secret key is disabled")
	}

	return key, nil
}
