package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"ffmpegserver/config"
	"ffmpegserver/model"
	"ffmpegserver/public/sql"
	"ffmpegserver/service/device"
	"ffmpegserver/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// 支持跨域（Wails 前端可能不同源）
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsAuthMessage 客户端认证消息
type wsAuthMessage struct {
	Type    string        `json:"type"`
	Payload wsAuthPayload `json:"payload"`
}

type wsAuthPayload struct {
	Token  string `json:"token"`
	PCCode string `json:"pc_code"`
}

// wsClientMessage 客户端通用消息
type wsClientMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// StartWsServer 启动 WebSocket 独立 HTTP 服务
func StartWsServer(port int) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// 跨域中间件
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	r.GET("/ws", wsHandler)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("[WsServer] 启动 WebSocket 服务 %s", addr)

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	for {
		var err error
		if os.Getenv("ENV") == "prod" {
			fmt.Println("SslCertificate：", config.Config.Ws.SslCertificate, config.Config.Ws.SslCertificateKey)
			//err = srv.ListenAndServeTLS(config.Config.Ws.SslCertificate, config.Config.Ws.SslCertificateKey)
			err = srv.ListenAndServe()
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[WsServer] 服务异常: %v，3秒后重试", err)
			time.Sleep(3 * time.Second)
		}
	}
}

// wsHandler WebSocket 升级握手 + JWT 认证 + 注册到 Hub
func wsHandler(c *gin.Context) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WsServer] WebSocket 升级失败: %v", err)
		return
	}

	// 5 秒内必须完成认证
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	var userId int32
	var pcCode string

	authenticated := false
	for !authenticated {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WsServer] 等待认证消息失败: %v", err)
			conn.Close()
			return
		}

		var authMsg wsAuthMessage
		if err := json.Unmarshal(msg, &authMsg); err != nil || authMsg.Type != "auth" {
			continue
		}

		// 解析 JWT
		claims, err := utils.ParseToken(authMsg.Payload.Token)
		if err != nil {
			writeJSON(conn, "auth_fail", map[string]string{"message": "Token 无效或已过期"})
			conn.Close()
			return
		}

		if claims.TokenType != "access" {
			writeJSON(conn, "auth_fail", map[string]string{"message": "请使用 Access Token"})
			conn.Close()
			return
		}

		userId = claims.UserID
		pcCode = authMsg.Payload.PCCode
		if pcCode == "" {
			writeJSON(conn, "auth_fail", map[string]string{"message": "缺少 pc_code"})
			conn.Close()
			return
		}
		authenticated = true
	}

	// 认证成功，清除读超时
	conn.SetReadDeadline(time.Time{})

	client := &WsClient{
		UserID: userId,
		PCCode: pcCode,
		Conn:   conn,
		Send:   make(chan []byte, 128),
	}

	// 自动注册/更新设备
	autoRegisterDevice(userId, pcCode, c.ClientIP())

	// 通知前端认证成功
	writeJSON(conn, "auth_success", map[string]interface{}{
		"user_id": userId,
		"pc_code": pcCode,
	})

	// 注册到 Hub
	GlobalWsHub.register <- client
	log.Printf("[WsServer] 用户 %d (PC:%s) WS 连接已注册", userId, pcCode)

	// 启动写泵和读泵
	go writePump(client)
	readPump(client)
}

// readPump 从 WS 连接读取消息
func readPump(client *WsClient) {
	defer func() {
		GlobalWsHub.unregister <- client
		client.Conn.Close()
	}()

	// 心跳：60 秒读超时
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, msg, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WsServer] 用户 %d 连接异常关闭: %v", client.UserID, err)
			}
			break
		}

		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// 解析并分发消息
		var clientMsg wsClientMessage
		if err := json.Unmarshal(msg, &clientMsg); err != nil {
			log.Printf("[WsServer] 消息解析失败: %v", err)
			continue
		}

		// 消息路由
		routeMessage(client, clientMsg.Type, clientMsg.Payload)
	}
}

// writePump 向 WS 连接写入消息
func writePump(client *WsClient) {
	ticker := time.NewTicker(30 * time.Second) // 30s ping
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := client.WriteJSON(msg); err != nil {
				log.Printf("[WsServer] 用户 %d 写入失败: %v", client.UserID, err)
				return
			}
		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// routeMessage 消息路由分发
func routeMessage(client *WsClient, msgType string, payload json.RawMessage) {
	switch msgType {
	case "ping":
		// 心跳回复由 readPump 的 pong handler 处理
		// 客户端主动 ping 只需要重置超时
		return

	case "dedup_progress":
		// 去重任务进度 → 交给调度器处理
		handleDedupProgress(client, payload)
	case "dedup_complete":
		handleDedupComplete(client, payload)
	case "dedup_error":
		handleDedupError(client, payload)
	case "dedup_log":
		handleDedupLog(client, payload)

	default:
		log.Printf("[WsServer] 未知消息类型: %s (userId=%d)", msgType, client.UserID)
	}
}

// autoRegisterDevice WS 认证成功后自动注册设备
func autoRegisterDevice(userId int32, pcCode, ip string) {
	var dev model.PcDevice
	result := sql.Gdb.Where("pc_code = ?", pcCode).First(&dev)

	nowUnix := time.Now().Unix()

	if result.Error != nil {
		// 设备不存在 → 自动创建
		name := pcCode
		if len(name) > 8 {
			name = "PC-" + name[:8]
		}
		dev = model.PcDevice{
			UserID:     userId,
			PCCode:     pcCode,
			DeviceName: name,
			IP:         ip,
			IsCurrent:  true,
			LastActive: nowUnix,
			CreatedAt:  nowUnix,
		}
		sql.Gdb.Create(&dev)
		log.Printf("[WsServer] 新设备自动注册: %s (user=%d, ip=%s)", pcCode, userId, ip)
	} else {
		// 设备已存在 → 更新 IP 和活跃时间
		sql.Gdb.Model(&dev).Updates(map[string]interface{}{
			"ip":          ip,
			"last_active": nowUnix,
		})
		log.Printf("[WsServer] 设备更新: %s (user=%d, ip=%s)", pcCode, userId, ip)
	}

	// 通知 service 层
	device.OnDeviceConnected(userId, pcCode, ip)
}

// writeJSON 向连接发送 JSON 消息
func writeJSON(conn *websocket.Conn, msgType string, payload interface{}) {
	data, _ := json.Marshal(WsMessage{Type: msgType, Payload: payload})
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	conn.WriteMessage(websocket.TextMessage, data)
}
