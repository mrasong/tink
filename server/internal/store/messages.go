package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	bbolt "go.etcd.io/bbolt"
)

var (
	ErrMessageNotFound = errors.New("message not found")
)

// Message 核心消息实体 (扁平化，移除 UserID，保留 KeyID)
type Message struct {
	ID        uint64         `json:"id"`
	KeyID     string         `json:"key_id,omitempty"` // 发送此消息的 Secret Key ID
	Devices   []string       `json:"devices"`          // 目标设备 ID 列表 (必填)
	Group     string         `json:"group,omitempty"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	URL       string         `json:"url,omitempty"`
	Sound     string         `json:"sound,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	CreatedAt int64          `json:"created_at"`
	ExpiresAt int64          `json:"expires_at"`
}

// NextMessageID 获取全局递增的消息 ID
func (s *Store) NextMessageID() (uint64, error) {
	var id uint64
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(BucketMessages)
		seq, err := b.NextSequence()
		if err != nil {
			return err
		}
		id = seq
		return nil
	})
	return id, err
}

// SaveMessage 保存消息并建立设备、分组、Key 索引
func (s *Store) SaveMessage(msg *Message) error {
	now := time.Now()
	if msg.CreatedAt == 0 {
		msg.CreatedAt = UnixMillis(now)
	}
	if msg.ExpiresAt == 0 {
		// 默认 7 天过期
		msg.ExpiresAt = UnixMillis(now.Add(7 * 24 * time.Hour))
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bMessages := tx.Bucket(BucketMessages)
		bKeyIdx := tx.Bucket(BucketIdxKeyMessages)
		bDeviceIdx := tx.Bucket(BucketIdxDeviceMessages)
		bGroupIdx := tx.Bucket(BucketIdxGroupMessages)

		idBytes := Itob(msg.ID)

		// 1. 保存消息本体
		if err := bMessages.Put(idBytes, data); err != nil {
			return err
		}

		// 2. 保存 Key 索引: key_id + message_id -> message_id
		if msg.KeyID != "" {
			keyIdxKey := append([]byte(msg.KeyID+":"), idBytes...)
			if err := bKeyIdx.Put(keyIdxKey, idBytes); err != nil {
				return err
			}
		}

		// 3. 保存设备索引: device_id + message_id -> message_id
		for _, devID := range msg.Devices {
			devKey := append([]byte(devID+":"), idBytes...)
			if err := bDeviceIdx.Put(devKey, idBytes); err != nil {
				return err
			}
		}

		// 4. 保存 Group 索引: group + message_id -> message_id
		if msg.Group != "" {
			groupKey := append([]byte(msg.Group+":"), idBytes...)
			if err := bGroupIdx.Put(groupKey, idBytes); err != nil {
				return err
			}
		}

		return nil
	})
}

// GetMessage 根据 ID 获取消息
func (s *Store) GetMessage(id uint64) (*Message, error) {
	var msg Message
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(BucketMessages)
		data := b.Get(Itob(id))
		if data == nil {
			return ErrMessageNotFound
		}
		return json.Unmarshal(data, &msg)
	})
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// GetMessagesAfter 查询指定设备在 afterID 之后的所有未过期消息 (用于 SSE 离线重放)
func (s *Store) GetMessagesAfter(deviceID string, afterID uint64, limit int) ([]*Message, error) {
	if limit <= 0 {
		limit = 100
	}
	var messages []*Message
	err := s.db.View(func(tx *bbolt.Tx) error {
		bMessages := tx.Bucket(BucketMessages)
		bDeviceIdx := tx.Bucket(BucketIdxDeviceMessages)

		prefix := []byte(deviceID + ":")
		c := bDeviceIdx.Cursor()

		// 游标寻找 prefix + afterID + 1 的起始位置
		startKey := append(prefix, Itob(afterID+1)...)
		k, _ := c.Seek(startKey)

		for ; k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			if len(k) < len(prefix)+8 {
				continue
			}
			msgIDBytes := k[len(prefix):]
			msgData := bMessages.Get(msgIDBytes)
			if msgData == nil {
				continue
			}

			var msg Message
			if err := json.Unmarshal(msgData, &msg); err == nil {
				// 过滤过期消息
				if NowMillis() < msg.ExpiresAt {
					messages = append(messages, &msg)
					if len(messages) >= limit {
						break
					}
				}
			}
		}
		return nil
	})

	return messages, err
}

// CleanExpiredMessages 定期清理所有桶中的过期消息及相应索引
func (s *Store) CleanExpiredMessages() (int, error) {
	deletedCount := 0

	err := s.db.Update(func(tx *bbolt.Tx) error {
		bMessages := tx.Bucket(BucketMessages)
		bKeyIdx := tx.Bucket(BucketIdxKeyMessages)
		bDeviceIdx := tx.Bucket(BucketIdxDeviceMessages)
		bGroupIdx := tx.Bucket(BucketIdxGroupMessages)

		c := bMessages.Cursor()
		for k, v := c.First(); k != nil; {
			var msg Message
			if err := json.Unmarshal(v, &msg); err == nil {
				if NowMillis() > msg.ExpiresAt {
					// 级联删除索引
					idBytes := Itob(msg.ID)
					if msg.KeyID != "" {
						keyIdxKey := append([]byte(msg.KeyID+":"), idBytes...)
						_ = bKeyIdx.Delete(keyIdxKey)
					}

					for _, devID := range msg.Devices {
						devKey := append([]byte(devID+":"), idBytes...)
						_ = bDeviceIdx.Delete(devKey)
					}

					if msg.Group != "" {
						groupKey := append([]byte(msg.Group+":"), idBytes...)
						_ = bGroupIdx.Delete(groupKey)
					}

					// 删除消息本体并继续
					nextK, nextV := c.Next()
					_ = bMessages.Delete(k)
					deletedCount++
					k, v = nextK, nextV
					continue
				}
			}
			k, v = c.Next()
		}
		return nil
	})

	return deletedCount, err
}
