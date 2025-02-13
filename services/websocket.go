package services

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/qianlnk/werewolf/models"
)

// WebSocketManager WebSocket连接管理器
type WebSocketManager struct {
	connections   map[string]*websocket.Conn // playerID -> connection
	connectionIDs map[string]string          // playerID -> connectionID
	rooms         map[string][]string        // roomID -> []playerID
	mutex         sync.RWMutex
	roomManager   *RoomManager
}

// NewWebSocketManager 创建WebSocket管理器实例
func NewWebSocketManager(rm *RoomManager) *WebSocketManager {
	return &WebSocketManager{
		connections:   make(map[string]*websocket.Conn),
		connectionIDs: make(map[string]string),
		rooms:         make(map[string][]string),
		roomManager:   rm,
	}
}

// RegisterConnection 注册新的WebSocket连接
func (wm *WebSocketManager) RegisterConnection(playerID string, conn *websocket.Conn, connectionID string) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	// 检查并清理该玩家的所有旧连接
	if oldConn, exists := wm.connections[playerID]; exists {
		// 直接关闭旧连接
		oldConn.Close()
		delete(wm.connections, playerID)
		delete(wm.connectionIDs, playerID)
	}

	// 保存新连接和连接ID
	wm.connections[playerID] = conn
	wm.connectionIDs[playerID] = connectionID

	// 启动消息处理协程
	go wm.handleMessages(playerID, conn)
}

// Message WebSocket消息结构
type Message struct {
	Type    string      `json:"type"`
	RoomID  string      `json:"room_id"`
	Content interface{} `json:"content"`
}

// JoinRoom 将玩家加入房间的WebSocket广播组
func (wm *WebSocketManager) JoinRoom(roomID, playerID string) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	// 检查房间是否存在
	if _, exists := wm.rooms[roomID]; !exists {
		wm.rooms[roomID] = make([]string, 0)
	}

	// 检查玩家是否已在房间中
	for _, pid := range wm.rooms[roomID] {
		if pid == playerID {
			// 玩家已在房间中，直接返回
			return
		}
	}

	// 玩家不在房间中，添加到房间
	wm.rooms[roomID] = append(wm.rooms[roomID], playerID)

	// 广播房间成员更新消息
	go func() {
		room, err := wm.roomManager.GetRoom(roomID)
		if err == nil {
			wm.BroadcastToRoom(roomID, map[string]interface{}{
				"type":    "room_update",
				"players": room.Players,
			})
		}
	}()
}

// BroadcastToRoom 向房间内所有玩家广播消息
func (wm *WebSocketManager) BroadcastToRoom(roomID string, message interface{}) {
	log.Printf("[WebSocket广播] 开始向房间 %s 广播消息, %v", roomID, message)

	// 序列化消息
	msgBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("[WebSocket广播] 消息序列化失败: %v", err)
		return
	}

	// 获取房间内的所有玩家ID
	wm.mutex.RLock()
	playerIDs, exists := wm.rooms[roomID]
	if !exists {
		wm.mutex.RUnlock()
		log.Printf("[WebSocket广播] 房间 %s 不存在", roomID)
		return
	}

	// 获取玩家的连接
	connections := make([]*websocket.Conn, 0)
	for _, playerID := range playerIDs {
		if conn, ok := wm.connections[playerID]; ok {
			connections = append(connections, conn)
		}
	}
	wm.mutex.RUnlock()

	log.Printf("[WebSocket广播] 房间 %s 中有 %d 个活跃连接", roomID, len(connections))

	// 向每个连接发送消息
	for _, conn := range connections {
		if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
			log.Printf("[WebSocket广播] 向连接发送消息失败: %v", err)
			continue
		}
	}

	log.Printf("[WebSocket广播] 消息广播完成")
}

// SendToPlayer 向指定玩家发送消息
func (wm *WebSocketManager) SendToPlayer(playerID string, message interface{}) error {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	conn, exists := wm.connections[playerID]
	if !exists {
		return errors.New("玩家未连接")
	}

	msg := Message{
		Type:    "private",
		Content: message,
	}

	// 使用重试机制发送消息
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if err := wm.checkConnection(conn); err != nil {
			if i == maxRetries-1 {
				go wm.RemoveConnection(playerID)
				return errors.New("连接已断开")
			}
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}

		// 设置写入超时
		conn.SetWriteDeadline(time.Now().Add(time.Second * 5))
		err := conn.WriteJSON(msg)
		conn.SetWriteDeadline(time.Time{})

		if err == nil {
			return nil // 发送成功
		}

		if i == maxRetries-1 {
			go wm.RemoveConnection(playerID)
			return err
		}

		log.Printf("发送消息到玩家 %s 失败 (尝试 %d/%d): %v", playerID, i+1, maxRetries, err)
		time.Sleep(time.Second * time.Duration(i+1))
	}

	return errors.New("发送消息失败，已达到最大重试次数")
}

// startPingHandler 启动心跳检测
func (wm *WebSocketManager) startPingHandler(playerID string, conn *websocket.Conn) {
	ticker := time.NewTicker(time.Second * 15) // 减少心跳间隔以更快检测连接问题
	defer ticker.Stop()

	maxFailures := 3
	failures := 0
	backoffLimit := 30 * time.Second

	for range ticker.C {
		// 先检查连接状态
		if conn == nil || (conn != nil && conn.WriteMessage == nil) {
			log.Printf("玩家 %s 的连接已失效", playerID)
			wm.RemoveConnection(playerID)
			return
		}

		// 设置写入超时
		if err := conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
			log.Printf("设置写入超时失败: %v", err)
			wm.RemoveConnection(playerID)
			return
		}

		// 发送ping消息
		if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second)); err != nil {
			failures++
			log.Printf("心跳检测失败 (%d/%d): %v", failures, maxFailures, err)

			if failures >= maxFailures {
				log.Printf("玩家 %s 的连接已断开（心跳检测失败达到上限）", playerID)
				wm.RemoveConnection(playerID)
				return
			}

			// 使用指数退避算法进行重试
			backoff := time.Duration(1<<uint(failures-1)) * time.Second
			if backoff > backoffLimit {
				backoff = backoffLimit
			}
			time.Sleep(backoff)
			continue
		}

		// 重置失败计数和写入超时
		failures = 0
		conn.SetWriteDeadline(time.Time{})
	}
}

// checkConnection 检查连接状态
func (wm *WebSocketManager) checkConnection(conn *websocket.Conn) error {
	if conn == nil {
		return errors.New("连接为空")
	}

	// 发送ping消息检查连接状态
	if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second)); err != nil {
		return err
	}
	return nil
}

// 添加延迟清理的时间常量
const playerCleanupDelay = 30 * time.Second

// RemoveConnection 移除WebSocket连接
func (wm *WebSocketManager) RemoveConnection(playerID string) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	// 获取连接
	conn, exists := wm.connections[playerID]
	if !exists {
		return
	}

	// 先检查连接状态
	if err := wm.checkConnection(conn); err == nil {
		// 连接仍然有效，尝试发送关闭消息
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "连接关闭")
		_ = conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		err := conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(100*time.Millisecond))
		if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			log.Printf("发送关闭消息失败: %v", err)
		}
	}

	// 从连接映射中删除
	delete(wm.connections, playerID)
	delete(wm.connectionIDs, playerID)

	// 确保连接被关闭
	conn.Close()

	// 设置一个重连窗口期，避免页面刷新时立即清理房间和玩家信息
	go func() {
		// 等待30秒，给玩家重连的机会
		time.Sleep(30 * time.Second)

		wm.mutex.Lock()
		defer wm.mutex.Unlock()

		// 检查玩家是否已经重新连接
		if _, reconnected := wm.connections[playerID]; reconnected {
			return
		}

		// 如果玩家没有重连，则清理房间信息
		for roomID, players := range wm.rooms {
			for i, pid := range players {
				if pid == playerID {
					// 广播玩家离开消息
					go wm.broadcastPlayerLeft(roomID, playerID)
					// 从房间中移除玩家
					wm.rooms[roomID] = append(players[:i], players[i+1:]...)
					break
				}
			}

			// 如果房间为空，清理房间
			if len(wm.rooms[roomID]) == 0 {
				delete(wm.rooms, roomID)
			}
		}

		log.Printf("玩家 %s 未在重连窗口期内重连，已清理房间资源", playerID)
	}()

	log.Printf("已清理玩家 %s 的连接资源，等待重连窗口期", playerID)
}

// broadcastPlayerLeft 广播玩家离开消息
func (wm *WebSocketManager) broadcastPlayerLeft(roomID, playerID string) {
	room, err := wm.roomManager.GetRoom(roomID)
	if err != nil {
		log.Printf("获取房间信息失败: %v", err)
		return
	}

	// 更新房间玩家列表
	for i, player := range room.Players {
		if player.ID == playerID {
			room.Players = append(room.Players[:i], room.Players[i+1:]...)
			break
		}
	}

	// 广播更新消息
	wm.BroadcastToRoom(roomID, map[string]interface{}{
		"type":      "player_left",
		"player_id": playerID,
		"players":   room.Players,
	})
}

// isPlayerInRoom 检查玩家是否在指定房间中
func (wm *WebSocketManager) isPlayerInRoom(roomID, playerID string) bool {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	players, exists := wm.rooms[roomID]
	if !exists {
		return false
	}

	for _, pid := range players {
		if pid == playerID {
			return true
		}
	}
	return false
}

// handleMessages 处理接收到的WebSocket消息
func (wm *WebSocketManager) handleMessages(playerID string, conn *websocket.Conn) {
	// 设置连接参数
	conn.SetReadLimit(512 * 1024) // 设置最大消息大小为512KB

	for {
		// 读取消息
		_, p, err := conn.ReadMessage()
		if err != nil {
			// 检查是否是正常的连接关闭
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("连接正常关闭: %v", err)
				break // 直接退出消息处理循环，不调用RemoveConnection
			}
			// 处理意外的连接关闭
			log.Printf("读取消息失败: %v", err)
			wm.RemoveConnection(playerID)
			break
		}

		// 解析消息
		var msg Message
		if err := json.Unmarshal(p, &msg); err != nil {
			log.Printf("解析消息失败: %v", err)
			continue
		}

		// 根据消息类型处理不同的业务逻辑
		switch msg.Type {
		case "game_action":
			// 打印完整的action消息内容
			log.Printf("收到game_action消息: RoomID=%s, PlayerID=%s, Content=%+v", msg.RoomID, playerID, msg.Content)

			// 验证房间ID
			if msg.RoomID == "" {
				wm.SendToPlayer(playerID, map[string]interface{}{
					"type":    "error",
					"message": "缺少房间ID",
				})
				continue
			}

			// 验证动作内容
			if action, ok := msg.Content.(map[string]interface{}); ok {
				// 检查必要字段是否存在且不为空
				actionType, typeOk := action["type"].(string)

				if !typeOk || actionType == "" {
					wm.SendToPlayer(playerID, map[string]interface{}{
						"type":    "error",
						"message": "无效的动作类型",
					})
					continue
				}

				// 对于开始游戏动作，直接处理
				if actionType == "start_game" {
					// 验证玩家是否在房间中
					if !wm.isPlayerInRoom(msg.RoomID, playerID) {
						wm.SendToPlayer(playerID, map[string]interface{}{
							"type":    "error",
							"message": "玩家不在房间中",
						})
						continue
					}

					// 获取游戏控制器并开始游戏
					if game, exists := wm.roomManager.GetGameController(msg.RoomID); exists {
						if err := game.StartGame(); err != nil {
							wm.SendToPlayer(playerID, map[string]interface{}{
								"type":    "error",
								"message": err.Error(),
							})
						}
					} else {
						wm.SendToPlayer(playerID, map[string]interface{}{
							"type":    "error",
							"message": "游戏未初始化",
						})
					}
					continue
				}

				// 其他游戏动作需要验证目标玩家
				targetID, targetOk := action["target"].(string)
				if !targetOk || targetID == "" {
					wm.SendToPlayer(playerID, map[string]interface{}{
						"type":    "error",
						"message": "无效的目标玩家",
					})
					continue
				}

				// 验证玩家是否在房间中
				if !wm.isPlayerInRoom(msg.RoomID, playerID) {
					wm.SendToPlayer(playerID, map[string]interface{}{
						"type":    "error",
						"message": "玩家不在房间中",
					})
					continue
				}

				// 验证目标玩家是否在房间中
				if !wm.isPlayerInRoom(msg.RoomID, targetID) {
					wm.SendToPlayer(playerID, map[string]interface{}{
						"type":    "error",
						"message": "目标玩家不在房间中",
					})
					continue
				}

				// 将动作转发给游戏控制器
				gameAction := models.GameAction{
					RoomID:   msg.RoomID,
					PlayerID: playerID,
					Type:     actionType,
					TargetID: targetID,
				}

				// 获取游戏控制器并处理动作
				if game, exists := wm.roomManager.GetGameController(msg.RoomID); exists {
					if err := game.ProcessAction(gameAction); err != nil {
						// 发送错误消息给玩家
						wm.SendToPlayer(playerID, map[string]interface{}{
							"type":    "error",
							"message": err.Error(),
						})
					}
				} else {
					wm.SendToPlayer(playerID, map[string]interface{}{
						"type":    "error",
						"message": "游戏未开始或不存在",
					})
				}
			}
		case "chat":
			// 处理聊天消息
			if chat, ok := msg.Content.(map[string]interface{}); ok {
				// 广播聊天消息给房间内所有玩家
				wm.BroadcastToRoom(msg.RoomID, map[string]interface{}{
					"type":      "chat",
					"player_id": playerID,
					"message":   chat["message"],
				})
			}
		default:
			log.Printf("未知的消息类型: %s", msg.Type)
		}
	}
}

// SetRoomManager 设置房间管理器实例
func (wm *WebSocketManager) SetRoomManager(rm *RoomManager) {
	wm.roomManager = rm
}
