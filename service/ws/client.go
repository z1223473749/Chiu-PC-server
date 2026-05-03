package ws

import (
	"sync"

	"github.com/gorilla/websocket"
)

// WsClient 单个 WebSocket 连接
type WsClient struct {
	UserID int32
	PCCode string // 设备唯一标识
	Conn   *websocket.Conn
	Send   chan []byte
	mu     sync.Mutex // 保护 Conn 并发写
}

// WriteJSON 线程安全地写入 JSON 消息
func (c *WsClient) WriteJSON(msg []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteMessage(websocket.TextMessage, msg)
}
