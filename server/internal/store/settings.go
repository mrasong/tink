package store

import (
	"strings"

	bbolt "go.etcd.io/bbolt"
)

// 系统元信息 / 配置常量 Key
var (
	KeyBarkRelayEnabled = []byte("bark_relay_enabled")
	KeyBarkServerURL    = []byte("bark_server_url")
	KeyBarkRoutePath    = []byte("bark_route_path")
)

// SystemSettings 系统配置项
type SystemSettings struct {
	BarkRelayEnabled bool   `json:"bark_relay_enabled"`
	BarkServerURL    string `json:"bark_server_url"`
	BarkRoutePath    string `json:"bark_route_path"`
}

// DefaultSettings 默认系统配置
func DefaultSettings() *SystemSettings {
	return &SystemSettings{
		BarkRelayEnabled: false,
		BarkServerURL:    "https://api.day.app",
		BarkRoutePath:    "/bark-relay",
	}
}

// GetSettings 从 BucketMeta 中以单字段单值 KV 形式读取系统配置
func (s *Store) GetSettings() (*SystemSettings, error) {
	settings := DefaultSettings()

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(BucketMeta)
		if b == nil {
			return nil
		}

		if val := b.Get(KeyBarkRelayEnabled); val != nil {
			settings.BarkRelayEnabled = (string(val) == "true" || string(val) == "1")
		}
		if val := b.Get(KeyBarkServerURL); val != nil && len(val) > 0 {
			settings.BarkServerURL = string(val)
		}
		if val := b.Get(KeyBarkRoutePath); val != nil && len(val) > 0 {
			settings.BarkRoutePath = string(val)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 规范化路由前缀，确保以 / 开头且不以 / 结尾
	settings.BarkRoutePath = NormalizeRoutePath(settings.BarkRoutePath)

	return settings, nil
}

// UpdateSettings 将配置以 KV 扁平方式写入 BucketMeta
func (s *Store) UpdateSettings(settings *SystemSettings) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(BucketMeta)
		if b == nil {
			var err error
			b, err = tx.CreateBucketIfNotExists(BucketMeta)
			if err != nil {
				return err
			}
		}

		relayVal := "false"
		if settings.BarkRelayEnabled {
			relayVal = "true"
		}
		if err := b.Put(KeyBarkRelayEnabled, []byte(relayVal)); err != nil {
			return err
		}

		serverURL := strings.TrimRight(strings.TrimSpace(settings.BarkServerURL), "/")
		if serverURL == "" {
			serverURL = "https://api.day.app"
		}
		if err := b.Put(KeyBarkServerURL, []byte(serverURL)); err != nil {
			return err
		}

		routePath := NormalizeRoutePath(settings.BarkRoutePath)
		if err := b.Put(KeyBarkRoutePath, []byte(routePath)); err != nil {
			return err
		}

		return nil
	})
}

// NormalizeRoutePath 规范化路由路径前缀，例如 "bark-relay" -> "/bark-relay", "/bark-relay/" -> "/bark-relay"
func NormalizeRoutePath(path string) string {
	clean := strings.TrimSpace(path)
	if clean == "" || clean == "/" {
		return "/bark-relay"
	}
	clean = "/" + strings.Trim(clean, "/")
	return clean
}
