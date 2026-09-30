package sse

import (
	"net/http/httptest"
	"sync"
	"testing"
)

func newTestConnection(deviceID string) *Connection {
	recorder := httptest.NewRecorder()
	return NewConnection("key", deviceID, recorder, recorder)
}

func TestUnregisterConnectionDoesNotRemoveReplacement(t *testing.T) {
	hub := NewHub()
	defer hub.CloseAll()

	oldConn := newTestConnection("device-1")
	newConn := newTestConnection("device-1")
	hub.Register(oldConn)
	hub.Register(newConn)

	if hub.UnregisterConnection(oldConn) {
		t.Fatal("expected replaced connection to be ignored")
	}
	if !hub.IsDeviceConnected("device-1") {
		t.Fatal("replacement connection should remain active")
	}
	if !hub.UnregisterConnection(newConn) {
		t.Fatal("expected active connection to be unregistered")
	}
}

func TestDisconnectHandlerRunsAfterRemoval(t *testing.T) {
	hub := NewHub()
	defer hub.CloseAll()

	var mu sync.Mutex
	wasConnected := false
	hub.SetDisconnectHandler(func(deviceID string) {
		mu.Lock()
		defer mu.Unlock()
		wasConnected = hub.IsDeviceConnected(deviceID)
	})

	conn := newTestConnection("device-1")
	hub.Register(conn)
	if !hub.UnregisterConnection(conn) {
		t.Fatal("expected connection to be unregistered")
	}

	mu.Lock()
	defer mu.Unlock()
	if wasConnected {
		t.Fatal("disconnect handler ran before connection was removed")
	}
}
