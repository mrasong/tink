package store

import (
	"encoding/json"
	"errors"
	"time"

	bbolt "go.etcd.io/bbolt"
)

var (
	ErrDeviceNotFound = errors.New("device not found")
)

// Device 设备实体 (直接关联 KeyID)
type Device struct {
	ID                 string `json:"id"`
	KeyID              string `json:"key_id,omitempty"` // 注册/绑定该设备的 Secret Key ID
	Name               string `json:"name"`
	CreatedAt          int64  `json:"created_at"`
	LastConnectedAt    int64  `json:"last_connected_at"`
	LastDisconnectedAt int64  `json:"last_disconnected_at"`
}

// UpsertDevice 添加或更新设备
func (s *Store) UpsertDevice(dev *Device) error {
	now := time.Now()
	if dev.CreatedAt == 0 {
		dev.CreatedAt = UnixMillis(now)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(BucketDevices)
		// 重新注册时保留设备绑定和连接历史，但当前状态必须从离线开始。
		if existingData := b.Get([]byte(dev.ID)); existingData != nil {
			var existingDev Device
			if err := json.Unmarshal(existingData, &existingDev); err == nil {
				if dev.KeyID == "" {
					dev.KeyID = existingDev.KeyID
				}
				dev.LastConnectedAt = existingDev.LastConnectedAt
				dev.LastDisconnectedAt = existingDev.LastDisconnectedAt
			}
		}

		data, err := json.Marshal(dev)
		if err != nil {
			return err
		}
		return b.Put([]byte(dev.ID), data)
	})
}

// GetDevice 根据 ID 获取设备
func (s *Store) GetDevice(id string) (*Device, error) {
	var dev Device
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(BucketDevices)
		data := b.Get([]byte(id))
		if data == nil {
			return ErrDeviceNotFound
		}
		return json.Unmarshal(data, &dev)
	})
	if err != nil {
		return nil, err
	}
	return &dev, nil
}

// UpdateDeviceConnected updates connection status and the corresponding timestamp.
func (s *Store) UpdateDeviceConnected(id string, connected bool) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(BucketDevices)
		data := b.Get([]byte(id))
		if data == nil {
			return ErrDeviceNotFound
		}
		var dev Device
		if err := json.Unmarshal(data, &dev); err != nil {
			return err
		}
		if connected {
			dev.LastConnectedAt = NowMillis()
		} else {
			dev.LastDisconnectedAt = NowMillis()
		}
		updated, err := json.Marshal(dev)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), updated)
	})
}

// DeleteDevice 删除指定设备
func (s *Store) DeleteDevice(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(BucketDevices)
		return b.Delete([]byte(id))
	})
}

// ListDevices 获取系统中所有已注册设备 (Admin 可见)
func (s *Store) ListDevices() ([]*Device, error) {
	var devices []*Device
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(BucketDevices)
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var dev Device
			if err := json.Unmarshal(v, &dev); err == nil {
				devices = append(devices, &dev)
			}
		}
		return nil
	})
	return devices, err
}

// ListDevicesByKey 获取特定 Secret Key 所绑定的设备 (普通 SK 隔离)
func (s *Store) ListDevicesByKey(keyID string) ([]*Device, error) {
	var devices []*Device
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(BucketDevices)
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var dev Device
			if err := json.Unmarshal(v, &dev); err == nil {
				if dev.KeyID == keyID {
					devices = append(devices, &dev)
				}
			}
		}
		return nil
	})
	return devices, err
}
