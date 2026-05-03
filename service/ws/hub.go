package ws

import (
	"encoding/json"
	"log"
	"sync"
)

// WsMessage 统一消息外层
type WsMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// WsHub 全局 WebSocket 连接管理器
// 按 userID 索引，每个用户支持多连接（多设备/多标签页）
type WsHub struct {
	mu         sync.RWMutex
	clients    map[int32][]*WsClient // userId → 多连接
	register   chan *WsClient
	unregister chan *WsClient
}

// GlobalWsHub 全局单例
var GlobalWsHub = &WsHub{
	clients:    make(map[int32][]*WsClient),
	register:   make(chan *WsClient, 64),
	unregister: make(chan *WsClient, 64),
}

// Run 启动事件循环（应在独立 goroutine 中运行）
func (h *WsHub) Run() {
	log.Println("[WsHub] 启动事件循环")
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.UserID] = append(h.clients[client.UserID], client)
			count := len(h.clients[client.UserID])
			h.mu.Unlock()
			log.Printf("[WsHub] 用户 %d 连接 (PC:%s)，当前连接数: %d", client.UserID, client.PCCode, count)

		case client := <-h.unregister:
			h.mu.Lock()
			conns := h.clients[client.UserID]
			newConns := make([]*WsClient, 0, len(conns))
			for _, c := range conns {
				if c != client {
					newConns = append(newConns, c)
				}
			}
			if len(newConns) == 0 {
				delete(h.clients, client.UserID)
			} else {
				h.clients[client.UserID] = newConns
			}
			h.mu.Unlock()
			close(client.Send)
			log.Printf("[WsHub] 用户 %d 断开连接 (PC:%s)", client.UserID, client.PCCode)
		}
	}
}

// PushToUser 向指定用户的所有连接推送消息
func (h *WsHub) PushToUser(userId int32, msgType string, payload interface{}) {
	data, err := json.Marshal(WsMessage{Type: msgType, Payload: payload})
	if err != nil {
		log.Printf("[WsHub] JSON 序列化失败: %v", err)
		return
	}

	h.mu.RLock()
	conns := h.clients[userId]
	h.mu.RUnlock()

	if len(conns) == 0 {
		log.Printf("[WsHub] 用户 %d 无在线连接，消息丢弃 (type=%s)", userId, msgType)
		return
	}

	for _, c := range conns {
		select {
		case c.Send <- data:
		default:
			// 发送缓冲满，跳过（不阻塞）
			log.Printf("[WsHub] 用户 %d (PC:%s) 发送缓冲满，跳过一条消息", userId, c.PCCode)
		}
	}
}

// PushToUserByPC 向指定用户的指定设备推送消息
func (h *WsHub) PushToUserByPC(userId int32, pcCode string, msgType string, payload interface{}) {
	data, err := json.Marshal(WsMessage{Type: msgType, Payload: payload})
	if err != nil {
		log.Printf("[WsHub] JSON 序列化失败: %v", err)
		return
	}

	h.mu.RLock()
	conns := h.clients[userId]
	h.mu.RUnlock()

	for _, c := range conns {
		if c.PCCode != pcCode {
			continue
		}
		select {
		case c.Send <- data:
		default:
			log.Printf("[WsHub] 用户 %d (PC:%s) 发送缓冲满，跳过一条消息", userId, pcCode)
		}
	}
}

// PushToAllExceptPC 向指定用户除指定 PC 外的所有连接推送
func (h *WsHub) PushToAllExceptPC(userId int32, excludePCCode string, msgType string, payload interface{}) {
	data, err := json.Marshal(WsMessage{Type: msgType, Payload: payload})
	if err != nil {
		log.Printf("[WsHub] JSON 序列化失败: %v", err)
		return
	}

	h.mu.RLock()
	conns := h.clients[userId]
	h.mu.RUnlock()

	for _, c := range conns {
		if c.PCCode == excludePCCode {
			continue
		}
		select {
		case c.Send <- data:
		default:
			log.Printf("[WsHub] 用户 %d (PC:%s) 发送缓冲满，跳过一条消息", userId, c.PCCode)
		}
	}
}
