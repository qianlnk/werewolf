package services

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/qianlnk/werewolf/models"
)

var (
	ErrRoomNotFound = errors.New("房间不存在")
	ErrRoomFull     = errors.New("房间已满")
)

// RoomManager 房间管理器
type RoomManager struct {
	rooms        map[string]*models.Room
	games        map[string]*GameController
	webSocketMgr *WebSocketManager
	mutex        sync.RWMutex
}

// NewRoomManager 创建房间管理器实例
func NewRoomManager(webSocketMgr *WebSocketManager) *RoomManager {
	return &RoomManager{
		rooms:        make(map[string]*models.Room),
		games:        make(map[string]*GameController),
		webSocketMgr: webSocketMgr,
	}
}

// CreateRoom 创建新房间
func (rm *RoomManager) CreateRoom(name string, mode models.GameMode, maxPlayers int) *models.Room {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	room := &models.Room{
		ID:         generateID(),
		Name:       name,
		Mode:       mode,
		MaxPlayers: maxPlayers,
		MinPlayers: 1, // 修改最小玩家数为1，允许更灵活的配置
		Players:    make([]models.Player, 0),
		CreatedAt:  time.Now().Unix(),
	}

	rm.rooms[room.ID] = room

	// 初始化游戏状态和控制器
	gameState := NewGameState(*room, rm)
	gameController := NewGameController(gameState, rm.webSocketMgr) // 传入WebSocket管理器实例
	rm.games[room.ID] = gameController

	return room
}

// GetRoom 获取房间信息
func (rm *RoomManager) GetRoom(roomID string) (*models.Room, error) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	room, exists := rm.rooms[roomID]
	if !exists {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

// ListRooms 获取所有房间列表
func (rm *RoomManager) ListRooms() []*models.Room {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	rooms := make([]*models.Room, 0, len(rm.rooms))
	for _, room := range rm.rooms {
		rooms = append(rooms, room)
	}
	return rooms
}

// JoinRoom 加入房间
func (rm *RoomManager) JoinRoom(roomID string, player models.Player) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	room, exists := rm.rooms[roomID]
	if !exists {
		return ErrRoomNotFound
	}

	if len(room.Players) >= room.MaxPlayers {
		return ErrRoomFull
	}

	// 检查玩家是否已在房间中
	for _, p := range room.Players {
		if p.ID == player.ID {
			// 玩家已在房间中，更新玩家信息
			p.Name = player.Name
			return nil
		}
	}

	room.Players = append(room.Players, player)

	// 更新游戏控制器中的玩家信息
	if game, exists := rm.games[roomID]; exists {
		game.game.Players = room.Players
	}

	return nil
}

// generateID 生成唯一ID
func generateID() string {
	// 这里使用时间戳作为简单的ID生成方式
	// 实际项目中应该使用更可靠的ID生成算法
	return time.Now().Format("20060102150405")
}

// GetGameController 获取游戏控制器
func (rm *RoomManager) GetGameController(roomID string) (*GameController, bool) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	game, exists := rm.games[roomID]
	return game, exists
}

// GetPlayer 获取房间中的玩家信息
func (rm *RoomManager) GetPlayer(roomID string, playerID string) (*models.Player, error) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	room, exists := rm.rooms[roomID]
	if !exists {
		return nil, ErrRoomNotFound
	}

	log.Printf("room: %v", room.Players)
	for _, player := range room.Players {
		if player.ID == playerID {
			return &player, nil
		}
	}

	return nil, errors.New("玩家不存在")
}
