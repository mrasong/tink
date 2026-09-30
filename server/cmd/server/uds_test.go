package main

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrasong/tink/server/internal/api"
	"github.com/mrasong/tink/server/internal/sse"
	"github.com/mrasong/tink/server/internal/store"
)

func TestUDSEndToEnd(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tink_uds_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "tink.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer st.Close()

	hub := sse.NewHub()
	mux := http.NewServeMux()
	handler := api.RegisterRoutes(mux, st, hub, api.Config{EnableDashboard: true})

	socketPath := filepath.Join(tempDir, "tink.sock")
	udsListener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to listen on uds: %v", err)
	}
	defer udsListener.Close()

	udsServer := &http.Server{Handler: api.UDSMarkerMiddleware(handler)}
	go func() {
		_ = udsServer.Serve(udsListener)
	}()
	defer udsServer.Close()

	// 使用 UDSClient 测试
	client := newUDSClient(socketPath)
	if !client.IsOnline() {
		t.Fatalf("expected online client")
	}

	// 1. List keys (初始应为空)
	keys, err := client.ListKeys()
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys, got %d", len(keys))
	}

	// 2. Create key via UDS
	rawToken, key, err := client.CreateKey("uds-key-1", "user")
	if err != nil {
		t.Fatalf("failed to create key via uds: %v", err)
	}
	if rawToken == "" || key == nil || key.Name != "uds-key-1" {
		t.Fatalf("invalid created key: %+v", key)
	}

	// 3. List keys again
	keys, err = client.ListKeys()
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}
	if len(keys) != 1 || keys[0].Name != "uds-key-1" {
		t.Fatalf("expected 1 key with name uds-key-1, got %+v", keys)
	}

	// 4. Disable key
	var dis uint8 = 0
	keyUpdated, err := client.UpdateKey(key.ID, nil, &dis, nil)
	if err != nil {
		t.Fatalf("failed to update key: %v", err)
	}
	if keyUpdated.Enabled != 0 {
		t.Fatalf("expected key disabled, got %d", keyUpdated.Enabled)
	}

	// 5. Delete key
	if err := client.DeleteKey(key.ID); err != nil {
		t.Fatalf("failed to delete key: %v", err)
	}

	// 6. Device 测试
	_ = st.UpsertDevice(&store.Device{
		ID:    "macbook-pro-m1",
		KeyID: "admin",
		Name:  "My MacBook Pro",
	})

	devices, err := client.ListDevices()
	if err != nil {
		t.Fatalf("failed to list devices: %v", err)
	}
	if len(devices) != 1 || devices[0].Name != "My MacBook Pro" {
		t.Fatalf("expected 1 device, got %+v", devices)
	}

	if err := client.DeleteDevice("macbook-pro-m1"); err != nil {
		t.Fatalf("failed to delete device: %v", err)
	}

	devices, _ = client.ListDevices()
	if len(devices) != 0 {
		t.Fatalf("expected 0 devices after delete, got %d", len(devices))
	}
}
