package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/mrasong/tink/server/internal/auth"
	"github.com/mrasong/tink/server/internal/store"
)

// ClientManager 统管在线（UDS Socket）与离线（BoltDB）两种访问模式
type ClientManager interface {
	Close() error
	IsOnline() bool

	// Secret Key 业务接口
	ListKeys() ([]*store.SecretKey, error)
	CreateKey(name, role string) (keyStr string, key *store.SecretKey, err error)
	UpdateKey(id string, name *string, enabled *uint8, role *string) (*store.SecretKey, error)
	DeleteKey(id string) error

	// Device 业务接口
	ListDevices() ([]*store.Device, error)
	DeleteDevice(id string) error
}

// 获取统一 Client（优先检测 UDS 在线，不通则打开本地 BoltDB）
func getClient() (ClientManager, error) {
	socketPath := filepath.Join(globalDataDir, "tink.sock")

	// 1. 尝试探测 UDS Socket 是否活跃 (超短 200ms 超时)
	conn, err := net.DialTimeout("unix", socketPath, 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		// UDS 可连通，使用在线 UDSClient
		return newUDSClient(socketPath), nil
	}

	// 2. UDS 不通，回退到离线打开 BoltDB
	st, err := openStore()
	if err != nil {
		return nil, fmt.Errorf("server is not running and failed to open local database: %w", err)
	}
	return newOfflineClient(st), nil
}

// -------------------------------------------------------------
// UDS 在线客户端
// -------------------------------------------------------------

type udsClient struct {
	client *http.Client
}

func newUDSClient(socketPath string) *udsClient {
	return &udsClient{
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", socketPath)
				},
			},
			Timeout: 5 * time.Second,
		},
	}
}

func (c *udsClient) Close() error {
	return nil
}

func (c *udsClient) IsOnline() bool {
	return true
}

func (c *udsClient) doRequest(method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, "http://unix"+path, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
		Error   string          `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &envelope); err == nil {
		if envelope.Code != 0 || envelope.Error != "" {
			msg := envelope.Message
			if msg == "" {
				msg = envelope.Error
			}
			return errors.New(msg)
		}

		if out != nil && len(envelope.Data) > 0 {
			if err := json.Unmarshal(envelope.Data, out); err != nil {
				return fmt.Errorf("failed to decode response data: %w", err)
			}
			return nil
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBytes))
	}

	if out != nil {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}
	return nil
}

func (c *udsClient) ListKeys() ([]*store.SecretKey, error) {
	var list []*store.SecretKey
	err := c.doRequest(http.MethodGet, "/api/v1/keys", nil, &list)
	return list, err
}

func (c *udsClient) CreateKey(name, role string) (string, *store.SecretKey, error) {
	var res struct {
		SecretKey string `json:"secret_key"`
		Token     string `json:"token"`
		ID        string `json:"id"`
		Name      string `json:"name"`
		Role      string `json:"role"`
		Enabled   uint8  `json:"enabled"`
		CreatedAt int64  `json:"created_at"`
	}
	req := map[string]string{"name": name, "role": role}
	if err := c.doRequest(http.MethodPost, "/api/v1/keys", req, &res); err != nil {
		return "", nil, err
	}
	rawKey := res.SecretKey
	if rawKey == "" {
		rawKey = res.Token
	}
	return rawKey, &store.SecretKey{
		ID:        res.ID,
		Name:      res.Name,
		Role:      res.Role,
		Enabled:   res.Enabled,
		CreatedAt: res.CreatedAt,
	}, nil
}

func (c *udsClient) UpdateKey(id string, name *string, enabled *uint8, role *string) (*store.SecretKey, error) {
	req := map[string]any{}
	if name != nil {
		req["name"] = *name
	}
	if enabled != nil {
		req["enabled"] = *enabled
	}
	if role != nil {
		req["role"] = *role
	}
	var res store.SecretKey
	if err := c.doRequest(http.MethodPut, "/api/v1/keys/"+id, req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *udsClient) DeleteKey(id string) error {
	return c.doRequest(http.MethodDelete, "/api/v1/keys/"+id, nil, nil)
}

func (c *udsClient) ListDevices() ([]*store.Device, error) {
	var list []*store.Device
	err := c.doRequest(http.MethodGet, "/api/v1/devices", nil, &list)
	return list, err
}

func (c *udsClient) DeleteDevice(id string) error {
	return c.doRequest(http.MethodDelete, "/api/v1/devices/"+id, nil, nil)
}

// -------------------------------------------------------------
// 离线客户端 (直接操作 bbolt)
// -------------------------------------------------------------

type offlineClient struct {
	st *store.Store
}

func newOfflineClient(st *store.Store) *offlineClient {
	return &offlineClient{st: st}
}

func (c *offlineClient) Close() error {
	return c.st.Close()
}

func (c *offlineClient) IsOnline() bool {
	return false
}

func (c *offlineClient) ListKeys() ([]*store.SecretKey, error) {
	return c.st.ListKeys()
}

func (c *offlineClient) CreateKey(name, role string) (string, *store.SecretKey, error) {
	rawToken, err := auth.GenerateToken()
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate secret key: %w", err)
	}

	if name == "" {
		name = "CLI Generated"
	}
	if role != "admin" {
		role = "user"
	}

	key := &store.SecretKey{
		ID:        "sk_" + rawToken[len(rawToken)-8:],
		Name:      name,
		Role:      role,
		Hash:      store.HashToken(rawToken),
		Enabled:   1,
		CreatedAt: time.Now().UnixMilli(),
	}

	if err := c.st.CreateKey(key); err != nil {
		return "", nil, fmt.Errorf("failed to save secret key: %w", err)
	}
	return rawToken, key, nil
}

func (c *offlineClient) UpdateKey(id string, name *string, enabled *uint8, role *string) (*store.SecretKey, error) {
	key, err := c.st.GetKeyByID(id)
	if err != nil {
		return nil, fmt.Errorf("secret key %s not found: %w", id, err)
	}
	if name != nil {
		key.Name = *name
	}
	if role != nil && (*role == "admin" || *role == "user") {
		key.Role = *role
	}
	if enabled != nil {
		if key.IsAdmin() && *enabled == 0 {
			return nil, fmt.Errorf("cannot disable Admin Secret Key: it is required for server administration")
		}
		key.Enabled = *enabled
	}
	if err := c.st.UpdateKey(key); err != nil {
		return nil, fmt.Errorf("failed to update secret key: %w", err)
	}
	return key, nil
}

func (c *offlineClient) DeleteKey(id string) error {
	key, err := c.st.GetKeyByID(id)
	if err != nil {
		return fmt.Errorf("secret key %s not found: %w", id, err)
	}
	if key.IsAdmin() {
		return fmt.Errorf("cannot delete Admin Secret Key: it is required for server administration")
	}
	return c.st.DeleteKey(id)
}

func (c *offlineClient) ListDevices() ([]*store.Device, error) {
	return c.st.ListDevices()
}

func (c *offlineClient) DeleteDevice(id string) error {
	return c.st.DeleteDevice(id)
}
