package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mrasong/tink/server/internal/store"
)

// Connection 代表单个 Mac 设备的 SSE 长连接
type Connection struct {
	KeyID       string
	DeviceID    string
	Writer      http.ResponseWriter
	Flusher     http.Flusher
	MsgChan     chan *store.Message
	CloseChan   chan struct{}
	Topics      map[string]bool
	mu          sync.RWMutex
	closed      bool
	ConnectedAt time.Time
}

// NewConnection 实例化连接
func NewConnection(keyID, deviceID string, w http.ResponseWriter, f http.Flusher) *Connection {
	return &Connection{
		KeyID:       keyID,
		DeviceID:    deviceID,
		Writer:      w,
		Flusher:     f,
		MsgChan:     make(chan *store.Message, 64),
		CloseChan:   make(chan struct{}),
		Topics:      make(map[string]bool),
		ConnectedAt: time.Now(),
	}
}

// SendMessage 向客户端输出标准 SSE 消息
func (c *Connection) SendMessage(msg *store.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("connection closed")
	}

	// 格式：
	// id: 1001
	// event: notification
	// data: {"title":"...","body":"..."}
	// \n
	_, err = fmt.Fprintf(c.Writer, "id: %d\nevent: notification\ndata: %s\n\n", msg.ID, string(data))
	if err != nil {
		return err
	}
	c.Flusher.Flush()
	return nil
}

// SendPing 发送 SSE 心跳保活帧
func (c *Connection) SendPing() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("connection closed")
	}

	_, err := fmt.Fprintf(c.Writer, "event: ping\ndata: {}\n\n")
	if err != nil {
		return err
	}
	c.Flusher.Flush()
	return nil
}

// Close 关闭连接通道
func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.CloseChan)
	}
}
