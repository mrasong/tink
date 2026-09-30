package store

import (
	"encoding/binary"
	"fmt"
	"time"

	bbolt "go.etcd.io/bbolt"
)

// Bucket 名称定义
var (
	BucketMeta       = []byte("meta")
	BucketDevices    = []byte("devices")
	BucketSecretKeys = []byte("secret_keys")
	BucketSKHashes   = []byte("sk_hashes")
	BucketMessages   = []byte("messages")
	BucketSessions   = []byte("sessions")

	// 索引 Bucket
	BucketIdxKeyMessages    = []byte("idx_key_messages")
	BucketIdxDeviceMessages = []byte("idx_device_messages")
	BucketIdxGroupMessages  = []byte("idx_group_messages")
)

// Store 封装 bbolt 数据库访问
type Store struct {
	db *bbolt.DB
}

// Open 打开或创建数据库文件，并初始化必要 buckets
func Open(path string) (*Store, error) {
	db, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bolt db failed: %w", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		buckets := [][]byte{
			BucketMeta,
			BucketDevices,
			BucketSecretKeys,
			BucketSKHashes,
			BucketMessages,
			BucketSessions,
			BucketIdxKeyMessages,
			BucketIdxDeviceMessages,
			BucketIdxGroupMessages,
		}
		for _, b := range buckets {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return fmt.Errorf("create bucket %s: %w", string(b), err)
			}
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close 关闭底层数据库
func (s *Store) Close() error {
	return s.db.Close()
}

// DB 暴露底层 bbolt 实例
func (s *Store) DB() *bbolt.DB {
	return s.db
}

// Itob 将 uint64 序列化为 8 字节大端序切片 (用于 bbolt key 保证顺序)
func Itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

// Btoi 将 8 字节大端序反序列化为 uint64
func Btoi(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(b)
}
