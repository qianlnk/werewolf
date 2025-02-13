package services

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/qianlnk/werewolf/models"
)

// GameController 游戏流程控制器
type GameController struct {
	game         *GameState
	stateMachine *StateMachine
	webSocket    *WebSocketManager
	timer        *time.Timer
	mutex        sync.RWMutex
}

// NewGameController 创建游戏控制器实例
func NewGameController(game *GameState, ws *WebSocketManager) *GameController {
	return &GameController{
		game:         game,
		stateMachine: NewStateMachine(game),
		webSocket:    ws,
	}
}

// StartGame 开始游戏
func (gc *GameController) StartGame() error {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	// 验证房间ID
	if gc.game.Room.ID == "" {
		return errors.New("无效的房间ID")
	}

	// 检查是否需要补充AI玩家
	if len(gc.game.Players) < 6 {
		// 保存现有玩家
		existingPlayers := make([]models.Player, len(gc.game.Players))
		copy(existingPlayers, gc.game.Players)

		// 计算需要补充的AI玩家数量
		aiCount := 6 - len(gc.game.Players)
		// 创建AI玩家
		for i := 0; i < aiCount; i++ {
			aiPlayer := models.Player{
				ID:    generateAIPlayerID(),
				Name:  generateAIPlayerName(i + 1),
				Type:  models.AIPlayer,
				Alive: true,
				Role:  models.Villager, // 初始设置为村民，后续会在分配角色时被重新设置
			}
			existingPlayers = append(existingPlayers, aiPlayer)
		}

		// 更新游戏和房间的玩家列表
		gc.game.Players = existingPlayers
		gc.game.Room.Players = existingPlayers

		// 更新房间管理器中的房间信息，确保AI玩家信息持久化
		if gc.game.roomManager != nil {
			if room, exists := gc.game.roomManager.rooms[gc.game.Room.ID]; exists {
				room.Players = existingPlayers
			}
		}

		// 广播房间玩家列表更新
		gc.webSocket.BroadcastToRoom(gc.game.Room.ID, map[string]interface{}{
			"type":    "room_update",
			"players": gc.game.Room.Players,
		})
	}

	// 设置房间游戏模式和最小玩家数
	gc.game.Room.Mode = models.ClassicMode
	gc.game.Room.MinPlayers = 6

	// 启动游戏并分配角色
	if err := gc.game.StartGame(); err != nil {
		return err
	}

	// 确保游戏状态已更新
	gc.game.IsStarted = true

	// 向每个玩家单独发送其角色信息
	for _, player := range gc.game.Players {
		gc.webSocket.SendToPlayer(player.ID, map[string]interface{}{
			"type":    "role_assigned",
			"role":    player.Role,
			"message": "游戏开始，你的角色是：" + string(player.Role),
		})
	}

	// 广播游戏开始消息，但不包含角色信息
	gc.webSocket.BroadcastToRoom(gc.game.Room.ID, map[string]interface{}{
		"type":    "game_started",
		"message": "游戏已开始",
	})

	// 启动游戏计时器
	gc.startPhaseTimer()

	// 广播游戏状态
	gc.broadcastGameState()

	return nil
}

// generateAIPlayerID 生成AI玩家ID
func generateAIPlayerID() string {
	now := time.Now()
	// 使用纳秒级时间戳和随机数确保唯一性
	return fmt.Sprintf("ai_%d_%d", now.UnixNano(), rand.Intn(1000))
}

// generateAIPlayerName 生成AI玩家名称
func generateAIPlayerName(index int) string {
	return fmt.Sprintf("AI玩家%d", index)
}

// ProcessAction 处理玩家动作
func (gc *GameController) ProcessAction(action models.GameAction) error {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	// 验证目标玩家是否存在且有效
	targetValid := false
	for _, player := range gc.game.Players {
		if player.ID == action.TargetID {
			targetValid = true
			break
		}
	}

	if !targetValid {
		return errors.New("无效的目标玩家")
	}

	// 验证并添加动作
	if err := gc.game.AddAction(action); err != nil {
		return err
	}

	// 处理动作结果
	processActionResult(gc.game, action)

	// 检查当前阶段是否可以结束
	if gc.stateMachine.isPhaseComplete() {
		if err := gc.endCurrentPhase(); err != nil {
			return err
		}
	} else {
		// 即使阶段未结束也要广播最新状态
		gc.broadcastGameState()
	}

	return nil
}

// processAIActions 处理AI玩家的行动
func (gc *GameController) processAIActions() {
	// 确保游戏已经开始
	if !gc.game.IsStarted {
		return
	}

	// 使用互斥锁确保线程安全
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	for _, player := range gc.game.Players {
		if player.Type == models.AIPlayer && player.Alive {
			// 创建AI玩家实例
			ai := NewAIPlayer(player.ID, player.Role, gc.game)
			// 获取AI的行动
			action := ai.DecideAction()
			// 处理AI的行动
			if err := gc.game.AddAction(action); err != nil {
				// 如果处理动作失败，记录错误并中断处理
				fmt.Printf("处理AI玩家 %s 的动作时出错: %v\n", player.ID, err)
				return
			}
			// 处理动作结果
			processActionResult(gc.game, action)
		}
	}

	// 检查当前阶段是否可以结束
	if gc.stateMachine.isPhaseComplete() {
		if err := gc.endCurrentPhase(); err != nil {
			fmt.Printf("结束当前阶段时出错: %v\n", err)
		}
	} else {
		// 即使阶段未结束也要广播最新状态
		gc.broadcastGameState()
	}
}

// startPhaseTimer 启动阶段计时器
func (gc *GameController) startPhaseTimer() {
	if gc.timer != nil {
		gc.timer.Stop()
	}

	// 处理AI玩家的行动
	gc.processAIActions()

	gc.timer = time.NewTimer(time.Duration(gc.game.TimeLeft) * time.Second)

	go func() {
		<-gc.timer.C
		gc.handlePhaseTimeout()
	}()
}

// handlePhaseTimeout 处理阶段超时
func (gc *GameController) handlePhaseTimeout() {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	// 强制结束当前阶段
	gc.endCurrentPhase()
}

// endCurrentPhase 结束当前阶段
func (gc *GameController) endCurrentPhase() error {
	// 转换游戏阶段
	if err := gc.stateMachine.TransitionPhase(); err != nil {
		// 检查是否是游戏结束的错误
		if err.Error() == VillagerWin || err.Error() == WerewolfWin {
			gc.handleGameEnd(err.Error())
			return nil
		}
		return err
	}

	// 重置计时器
	gc.startPhaseTimer()

	// 广播新阶段信息
	gc.broadcastGameState()

	return nil
}

// handleGameEnd 处理游戏结束
func (gc *GameController) handleGameEnd(result string) {
	// 停止计时器
	if gc.timer != nil {
		gc.timer.Stop()
	}

	// 广播游戏结果
	gc.webSocket.BroadcastToRoom(gc.game.Room.ID, map[string]interface{}{
		"type":    "game_end",
		"result":  result,
		"players": gc.game.Players,
	})
}

// broadcastGameState 广播游戏状态
func (gc *GameController) broadcastGameState() {
	log.Printf("[广播游戏状态] 房间ID: %s, 阶段: %s, 回合: %d", gc.game.Room.ID, gc.game.Phase, gc.game.Round)
	log.Printf("[广播游戏状态] 存活玩家: %d, 剩余时间: %d秒", countAlivePlayers(gc.game.Players), gc.game.TimeLeft)

	// 构建游戏状态消息
	gameState := map[string]interface{}{
		"type":       "game_state",
		"phase":      gc.game.Phase,
		"round":      gc.game.Round,
		"time_left":  gc.game.TimeLeft,
		"players":    gc.game.Players,
		"is_started": gc.game.IsStarted,
		"room":       gc.game.Room,
	}

	log.Printf("[广播游戏状态] 发送状态消息: %+v", gameState)

	// 直接广播游戏状态，不需要额外的包装
	gc.webSocket.BroadcastToRoom(gc.game.Room.ID, gameState)
}

// countAlivePlayers 统计存活玩家数量
func countAlivePlayers(players []models.Player) int {
	count := 0
	for _, player := range players {
		if player.Alive {
			count++
		}
	}
	return count
}
