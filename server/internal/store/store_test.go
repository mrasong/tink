package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrasong/tink/server/internal/store"
)

func setupTestDB(t *testing.T) (*store.Store, func()) {
	tmpDir, err := os.MkdirTemp("", "tink-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	dbPath := filepath.Join(tmpDir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to open test store: %v", err)
	}

	cleanup := func() {
		_ = st.Close()
		_ = os.RemoveAll(tmpDir)
	}
	return st, cleanup
}

func TestStoreSecretKey(t *testing.T) {
	st, cleanup := setupTestDB(t)
	defer cleanup()

	rawToken := "sk-tink-test12345678"
	key := &store.SecretKey{
		ID:        "sk-1",
		Name:      "Test Key",
		Role:      "admin",
		Enabled:   1,
		CreatedAt: time.Now().UnixMilli(),
		Hash:      store.HashToken(rawToken),
	}
	if err := st.CreateKey(key); err != nil {
		t.Fatalf("create key failed: %v", err)
	}

	foundKey, err := st.GetKeyByHash(store.HashToken(rawToken))
	if err != nil {
		t.Fatalf("get key by hash failed: %v", err)
	}
	if foundKey.Role != "admin" {
		t.Errorf("expected key role admin, got %s", foundKey.Role)
	}
	if foundKey.Enabled != 1 {
		t.Errorf("expected enabled 1, got %d", foundKey.Enabled)
	}
}

func TestMessageAndOfflineReplay(t *testing.T) {
	st, cleanup := setupTestDB(t)
	defer cleanup()

	devID := "device-1"

	// 保存设备
	err := st.UpsertDevice(&store.Device{
		ID:    devID,
		KeyID: "sk-1",
		Name:  "MacBook Pro",
	})
	if err != nil {
		t.Fatalf("upsert device failed: %v", err)
	}

	// 连续写入 5 条消息 (指定发给 device-1)
	for i := 1; i <= 5; i++ {
		msgID, err := st.NextMessageID()
		if err != nil {
			t.Fatalf("get next message id: %v", err)
		}
		msg := &store.Message{
			ID:        msgID,
			KeyID:     "sk-1",
			Devices:   []string{devID},
			Title:     "Title",
			Body:      "Body",
			CreatedAt: time.Now().UnixMilli(),
		}
		if err := st.SaveMessage(msg); err != nil {
			t.Fatalf("save message: %v", err)
		}
	}

	// 离线重放：获取 ID > 2 的消息 (期望 3, 4, 5)
	msgs, err := st.GetMessagesAfter(devID, 2, 10)
	if err != nil {
		t.Fatalf("get messages after: %v", err)
	}

	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].ID != 3 || msgs[1].ID != 4 || msgs[2].ID != 5 {
		t.Errorf("unexpected message IDs: %v, %v, %v", msgs[0].ID, msgs[1].ID, msgs[2].ID)
	}
}

func TestDeviceReconnectPreservesConnectionHistory(t *testing.T) {
	st, cleanup := setupTestDB(t)
	defer cleanup()

	if err := st.UpsertDevice(&store.Device{ID: "device-1", KeyID: "sk-1", Name: "Mac"}); err != nil {
		t.Fatalf("upsert device failed: %v", err)
	}
	if err := st.UpdateDeviceConnected("device-1", true); err != nil {
		t.Fatalf("mark device connected failed: %v", err)
	}
	if err := st.UpdateDeviceConnected("device-1", false); err != nil {
		t.Fatalf("mark device disconnected failed: %v", err)
	}

	before, err := st.GetDevice("device-1")
	if err != nil {
		t.Fatalf("get device before reconnect failed: %v", err)
	}
	if before.LastConnectedAt == 0 || before.LastDisconnectedAt == 0 {
		t.Fatalf("expected connection history before reconnect: %+v", before)
	}

	if err := st.UpsertDevice(&store.Device{ID: "device-1", KeyID: "sk-1", Name: "Updated Mac"}); err != nil {
		t.Fatalf("re-register device failed: %v", err)
	}
	after, err := st.GetDevice("device-1")
	if err != nil {
		t.Fatalf("get device after reconnect failed: %v", err)
	}
	if after.Name != "Updated Mac" {
		t.Fatalf("unexpected device after re-register: %+v", after)
	}
	if after.LastConnectedAt != before.LastConnectedAt || after.LastDisconnectedAt != before.LastDisconnectedAt {
		t.Fatalf("connection history was lost: before=%+v after=%+v", before, after)
	}
}

func TestSettings(t *testing.T) {
	st, cleanup := setupTestDB(t)
	defer cleanup()

	// 1. 读取默认配置
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatalf("get settings failed: %v", err)
	}
	if settings.BarkRelayEnabled {
		t.Errorf("expected default BarkRelayEnabled to be false, got %v", settings.BarkRelayEnabled)
	}
	if settings.BarkServerURL != "https://api.day.app" {
		t.Errorf("expected default BarkServerURL to be https://api.day.app, got %s", settings.BarkServerURL)
	}
	if settings.BarkRoutePath != "/bark-relay" {
		t.Errorf("expected default BarkRoutePath to be /bark-relay, got %s", settings.BarkRoutePath)
	}

	// 2. 更新配置
	updated := &store.SystemSettings{
		BarkRelayEnabled: true,
		BarkServerURL:    "https://bark.mycompany.com/",
		BarkRoutePath:    "my-relay/",
	}
	if err := st.UpdateSettings(updated); err != nil {
		t.Fatalf("update settings failed: %v", err)
	}

	// 3. 再次读取验证
	reloaded, err := st.GetSettings()
	if err != nil {
		t.Fatalf("reload settings failed: %v", err)
	}
	if !reloaded.BarkRelayEnabled {
		t.Errorf("expected BarkRelayEnabled to be true")
	}
	if reloaded.BarkServerURL != "https://bark.mycompany.com" {
		t.Errorf("expected BarkServerURL without trailing slash, got %s", reloaded.BarkServerURL)
	}
	if reloaded.BarkRoutePath != "/my-relay" {
		t.Errorf("expected BarkRoutePath normalized to /my-relay, got %s", reloaded.BarkRoutePath)
	}
}
