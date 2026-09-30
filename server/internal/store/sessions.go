package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	bbolt "go.etcd.io/bbolt"
)

var ErrSessionNotFound = errors.New("session not found")

// Session 浏览器会话实体 (httpOnly cookie 凭证，与长期 Secret Key 分离)
type Session struct {
	TokenHash   string `json:"token_hash"`   // 会话令牌 SHA-256 十六进制 (不落明文)
	KeyID       string `json:"key_id"`       // 登录时验证通过的 SecretKey ID
	CreatedAt   int64  `json:"created_at"`   // Unix 毫秒
	LastSeen    int64  `json:"last_seen"`    // Unix 毫秒，滑动空闲超时判定用
	AbsoluteExp int64  `json:"absolute_exp"` // Unix 毫秒，绝对上限，到期须重新输入 Secret Key
}

// Session 时效参数
const (
	SessionIdleTimeout     = 8 * time.Hour
	SessionAbsoluteTimeout = 24 * time.Hour
	// 续写 LastSeen 的最小间隔，避免每个请求都写 bbolt
	SessionTouchInterval = 60 * time.Second
)

// NewSessionToken 生成 256 位随机会话令牌 (十六进制)
func NewSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateSession 持久化新会话，返回入库的实体
func (s *Store) CreateSession(tokenHash, keyID string) (*Session, error) {
	now := NowMillis()
	sess := &Session{
		TokenHash:   tokenHash,
		KeyID:       keyID,
		CreatedAt:   now,
		LastSeen:    now,
		AbsoluteExp: now + SessionAbsoluteTimeout.Milliseconds(),
	}
	data, err := json.Marshal(sess)
	if err != nil {
		return nil, err
	}
	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(BucketSessions).Put([]byte(tokenHash), data)
	})
	if err != nil {
		return nil, err
	}
	return sess, nil
}

// GetSession 按令牌哈希查找会话；过期 (空闲超限或超绝对上限) 视为无效并删除。
// 超过 SessionTouchInterval 的 LastSeen 会被顺手续期。
func (s *Store) GetSession(tokenHash string) (*Session, error) {
	var sess Session
	var expired bool
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(BucketSessions)
		data := b.Get([]byte(tokenHash))
		if data == nil {
			return ErrSessionNotFound
		}
		if err := json.Unmarshal(data, &sess); err != nil {
			return err
		}
		now := NowMillis()
		expired = now-sess.LastSeen > SessionIdleTimeout.Milliseconds() ||
			now > sess.AbsoluteExp
		if expired {
			return b.Delete([]byte(tokenHash))
		}
		if now-sess.LastSeen > SessionTouchInterval.Milliseconds() {
			sess.LastSeen = now
			updated, err := json.Marshal(&sess)
			if err != nil {
				return err
			}
			return b.Put([]byte(tokenHash), updated)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if expired {
		return nil, ErrSessionNotFound
	}
	return &sess, nil
}

// DeleteSession 吊销会话 (登出)
func (s *Store) DeleteSession(tokenHash string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(BucketSessions).Delete([]byte(tokenHash))
	})
}

// CleanExpiredSessions 清理所有过期会话，返回清理条数
func (s *Store) CleanExpiredSessions() (int, error) {
	count := 0
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(BucketSessions)
		now := NowMillis()
		var toDelete [][]byte
		if err := b.ForEach(func(k, v []byte) error {
			var sess Session
			if err := json.Unmarshal(v, &sess); err != nil {
				toDelete = append(toDelete, append([]byte{}, k...))
				return nil
			}
			if now-sess.LastSeen > SessionIdleTimeout.Milliseconds() || now > sess.AbsoluteExp {
				toDelete = append(toDelete, append([]byte{}, k...))
			}
			return nil
		}); err != nil {
			return err
		}
		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		count = len(toDelete)
		return nil
	})
	return count, err
}
