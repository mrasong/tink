package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	bbolt "go.etcd.io/bbolt"
)

var (
	ErrSecretKeyNotFound = errors.New("secret key not found")
)

// SecretKey 认证密钥实体 (替代原 Token)
type SecretKey struct {
	ID        string `json:"id"`                  // 唯一标识 (如 sk_8a1b2c3d)
	Name      string `json:"name"`                // 备注名称 (如 "GitHub Actions", "Grafana", "Work Mac")
	Role      string `json:"role"`                // 角色: "admin" (主管理密钥) 或 "user" (普通业务密钥)
	Hash      string `json:"hash"`                // SHA-256 十六进制哈希值 (安全存储，不存明文)
	Enabled   uint8  `json:"enabled"`             // 是否启用: 1 为启用, 0 为禁用
	CreatedAt int64  `json:"created_at"`          // 创建时间（Unix 毫秒）
	LastUsed  int64  `json:"last_used,omitempty"` // 最近一次调用时间（Unix 毫秒）
}

// IsAdmin 辅助判定是否为管理员角色
func (k *SecretKey) IsAdmin() bool {
	return k.Role == "admin"
}

// HashToken 计算密钥明文的 SHA256 十六进制字符串
func HashToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}

// CreateKey 保存新 SecretKey (双写: BucketSecretKeys[ID]=Key, BucketSKHashes[Hash]=ID)
func (s *Store) CreateKey(k *SecretKey) error {
	if k.CreatedAt == 0 {
		k.CreatedAt = NowMillis()
	}
	data, err := json.Marshal(k)
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bKeys := tx.Bucket(BucketSecretKeys)
		bHashes := tx.Bucket(BucketSKHashes)

		// 检查 Hash 是否已存在且指向其他 Key
		if existingID := bHashes.Get([]byte(k.Hash)); existingID != nil && string(existingID) != k.ID {
			return errors.New("secret key hash already exists")
		}

		if err := bKeys.Put([]byte(k.ID), data); err != nil {
			return err
		}
		return bHashes.Put([]byte(k.Hash), []byte(k.ID))
	})
}

// GetKeyByID 通过 ID 查找 SecretKey
func (s *Store) GetKeyByID(id string) (*SecretKey, error) {
	var k SecretKey
	err := s.db.View(func(tx *bbolt.Tx) error {
		bKeys := tx.Bucket(BucketSecretKeys)
		data := bKeys.Get([]byte(id))
		if data == nil {
			return ErrSecretKeyNotFound
		}
		return json.Unmarshal(data, &k)
	})
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// GetKeyByHash 通过哈希查找 SecretKey 并更新其 LastUsed
func (s *Store) GetKeyByHash(hash string) (*SecretKey, error) {
	var k SecretKey
	err := s.db.Update(func(tx *bbolt.Tx) error {
		bKeys := tx.Bucket(BucketSecretKeys)
		bHashes := tx.Bucket(BucketSKHashes)

		keyID := bHashes.Get([]byte(hash))
		var data []byte
		if keyID != nil {
			data = bKeys.Get(keyID)
		} else {
			// 向下兼容旧格式 (如果曾直接以 hash 作为 Key 存入)
			data = bKeys.Get([]byte(hash))
			if data != nil {
				var legacyKey SecretKey
				if err := json.Unmarshal(data, &legacyKey); err == nil && legacyKey.ID != "" {
					_ = bKeys.Delete([]byte(hash))
					_ = bKeys.Put([]byte(legacyKey.ID), data)
					_ = bHashes.Put([]byte(hash), []byte(legacyKey.ID))
					keyID = []byte(legacyKey.ID)
				}
			}
		}

		if data == nil {
			return ErrSecretKeyNotFound
		}

		if err := json.Unmarshal(data, &k); err != nil {
			return err
		}

		k.LastUsed = NowMillis()
		updated, err := json.Marshal(k)
		if err != nil {
			return err
		}
		return bKeys.Put([]byte(k.ID), updated)
	})
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// ListKeys 获取所有 SecretKey 列表
func (s *Store) ListKeys() ([]*SecretKey, error) {
	var list []*SecretKey
	err := s.db.View(func(tx *bbolt.Tx) error {
		bKeys := tx.Bucket(BucketSecretKeys)
		c := bKeys.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var key SecretKey
			if err := json.Unmarshal(v, &key); err == nil {
				list = append(list, &key)
			}
		}
		return nil
	})
	return list, err
}

// UpdateKey 更新已存在的 SecretKey 实体
func (s *Store) UpdateKey(k *SecretKey) error {
	data, err := json.Marshal(k)
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bKeys := tx.Bucket(BucketSecretKeys)
		bHashes := tx.Bucket(BucketSKHashes)

		existing := bKeys.Get([]byte(k.ID))
		if existing == nil {
			return ErrSecretKeyNotFound
		}

		// 确保 Hash 索引也同步建立
		if k.Hash != "" {
			_ = bHashes.Put([]byte(k.Hash), []byte(k.ID))
		}

		return bKeys.Put([]byte(k.ID), data)
	})
}

// DeleteKey 删除指定 SecretKey 并清理 Hash 索引
func (s *Store) DeleteKey(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bKeys := tx.Bucket(BucketSecretKeys)
		bHashes := tx.Bucket(BucketSKHashes)

		data := bKeys.Get([]byte(id))
		if data == nil {
			return ErrSecretKeyNotFound
		}

		var k SecretKey
		if err := json.Unmarshal(data, &k); err == nil && k.Hash != "" {
			_ = bHashes.Delete([]byte(k.Hash))
		}

		return bKeys.Delete([]byte(id))
	})
}
