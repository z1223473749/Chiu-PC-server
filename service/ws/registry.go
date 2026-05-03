package ws

import (
	"encoding/json"
	"log"
)

// MessageHandler WS 消息处理器
type MessageHandler func(client *WsClient, payload json.RawMessage)

var messageHandlers = make(map[string]MessageHandler)

// RegisterMessageHandler 注册消息处理器（避免包循环依赖）
func RegisterMessageHandler(msgType string, handler MessageHandler) {
	if handler == nil {
		delete(messageHandlers, msgType)
		return
	}
	messageHandlers[msgType] = handler
	log.Printf("[WsRegistry] 注册消息处理器: %s", msgType)
}

// dispatchMessage 分发消息到已注册的处理器
func dispatchMessage(client *WsClient, msgType string, payload json.RawMessage) {
	handler, ok := messageHandlers[msgType]
	if !ok {
		log.Printf("[WsRegistry] 未注册的消息类型: %s", msgType)
		return
	}
	handler(client, payload)
}
