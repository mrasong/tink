package sse

import (
	"log"
	"sync"
	"time"

	"github.com/mrasong/tink/server/internal/store"
)

// Hub 管理所有在线设备的 SSE 连接 (按 DeviceID 唯一管理)
type Hub struct {
	mu           sync.RWMutex
	devices      map[string]*Connection
	stop         chan struct{}
	onDisconnect func(string)
}

func (h *Hub) SetDisconnectHandler(handler func(string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onDisconnect = handler
}

// NewHub 初始化 SSE 连接中心
func NewHub() *Hub {
	h := &Hub{
		devices: make(map[string]*Connection),
		stop:    make(chan struct{}),
	}
	go h.heartbeatLoop()
	return h
}

// Register 注册新设备长连接，若已有连接则优雅替换
func (h *Hub) Register(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if oldConn, exists := h.devices[conn.DeviceID]; exists {
		oldConn.Close()
	}

	h.devices[conn.DeviceID] = conn
	log.Printf("[Hub] Device connected: device=%s key=%s", conn.DeviceID, conn.KeyID)
}

// Unregister 注销设备连接
func (h *Hub) Unregister(deviceID string) {
	h.mu.Lock()
	conn, exists := h.devices[deviceID]
	if exists {
		conn.Close()
		delete(h.devices, deviceID)
		log.Printf("[Hub] Device disconnected: device=%s", deviceID)
	}
	handler := h.onDisconnect
	h.mu.Unlock()
	if exists && handler != nil {
		handler(deviceID)
	}
}

// UnregisterConnection removes conn only when it is still the active connection.
func (h *Hub) UnregisterConnection(conn *Connection) bool {
	h.mu.Lock()
	current, exists := h.devices[conn.DeviceID]
	if !exists || current != conn {
		h.mu.Unlock()
		return false
	}
	current.Close()
	delete(h.devices, conn.DeviceID)
	log.Printf("[Hub] Device disconnected: device=%s", conn.DeviceID)
	handler := h.onDisconnect
	h.mu.Unlock()
	if handler != nil {
		handler(conn.DeviceID)
	}
	return true
}

// IsDeviceConnected 检查指定设备是否处于实时在线状态
func (h *Hub) IsDeviceConnected(deviceID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, exists := h.devices[deviceID]
	return exists
}

// DispatchMessage 向目标设备分发推送消息 (已禁用广播，必须指定 target devices)
func (h *Hub) DispatchMessage(msg *store.Message) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(msg.Devices) == 0 || len(h.devices) == 0 {
		return 0
	}

	sentCount := 0
	for _, devID := range msg.Devices {
		if conn, exists := h.devices[devID]; exists {
			select {
			case conn.MsgChan <- msg:
				sentCount++
			default:
				log.Printf("[Hub] Dropping message to full buffer: device=%s msg_id=%d", devID, msg.ID)
			}
		}
	}

	return sentCount
}

// CloseAll 关闭所有连接与后台心跳协程
func (h *Hub) CloseAll() {
	close(h.stop)

	h.mu.Lock()
	connections := make([]string, 0, len(h.devices))
	for devID, conn := range h.devices {
		conn.Close()
		delete(h.devices, devID)
		connections = append(connections, devID)
	}
	handler := h.onDisconnect
	h.mu.Unlock()
	if handler != nil {
		for _, devID := range connections {
			handler(devID)
		}
	}
}

// heartbeatLoop 定期向客户端发送心跳帧 (每 30 秒)
func (h *Hub) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.stop:
			return
		case <-ticker.C:
			h.mu.RLock()
			for devID, conn := range h.devices {
				if err := conn.SendPing(); err != nil {
					log.Printf("[Hub] Heartbeat ping failed, disconnecting device=%s: %v", devID, err)
					go h.UnregisterConnection(conn)
				}
			}
			h.mu.RUnlock()
		}
	}
}
