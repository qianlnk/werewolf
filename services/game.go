package services

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/qianlnk/werewolf/models"
)

// 游戏阶段
const (
	PhaseNight = "night" // 夜晚阶段
	PhaseDay   = "day"   // 白天阶段
	PhaseVote  = "vote"  // 投票阶段
)

var (
	ErrGameNotStarted = errors.New("游戏尚未开始")
	ErrGameInProgress = errors.New("游戏正在进行中")
	ErrInvalidAction  = errors.New("无效的游戏动作")
	ErrInvalidPhase   = errors.New("当前阶段无法执行该动作")
)

// GameManager 游戏管理器
type GameManager struct {
	games map[string]*GameState
	mutex sync.RWMutex
}

// GameManager 使用 GameState 结构体
var _ = GameState{} // 确保 GameState 被正确导入

// NewGameManager 创建游戏管理器实例
func NewGameManager() *GameManager {
	return &GameManager{
		games: make(map[string]*GameState),
	}
}

// StartGame 开始游戏
func (gm *GameManager) StartGame(roomID string, players []models.Player) error {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	if _, exists := gm.games[roomID]; exists {
		return ErrGameInProgress
	}

	game := &GameState{
		RoomID:    roomID,
		Phase:     "night",
		Round:     1,
		Players:   players,
		Actions:   make([]models.GameAction, 0),
		TimeLeft:  120, // 每个阶段2分钟
		IsStarted: true,
	}

	// 分配角色
	assignRoles(game)

	gm.games[roomID] = game
	return nil
}

// GetGameStatus 获取游戏状态
func (gm *GameManager) GetGameStatus(roomID string) (*models.GameStatus, error) {
	gm.mutex.RLock()
	defer gm.mutex.RUnlock()

	game, exists := gm.games[roomID]
	if !exists || !game.IsStarted {
		return nil, ErrGameNotStarted
	}

	return &models.GameStatus{
		Phase:    game.Phase,
		Round:    game.Round,
		Players:  game.Players,
		Actions:  getAvailableActions(game),
		TimeLeft: game.TimeLeft,
	}, nil
}

// ProcessAction 处理游戏动作
func (gm *GameManager) ProcessAction(roomID string, action models.GameAction) error {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	game, exists := gm.games[roomID]
	if !exists || !game.IsStarted {
		return ErrGameNotStarted
	}

	// 验证动作是否有效
	if !isValidAction(game, action) {
		return ErrInvalidAction
	}

	// 记录动作
	game.Actions = append(game.Actions, action)

	// 使用状态机处理游戏状态转换
	sm := NewStateMachine(game)
	if err := sm.TransitionPhase(); err != nil {
		// 检查是否是游戏结束的错误
		if err.Error() == WerewolfWin || err.Error() == VillagerWin {
			// 处理游戏结束逻辑
			game.IsStarted = false
		}
		return err
	}

	// 记录动作
	game.Actions = append(game.Actions, action)

	// 处理动作结果
	processActionResult(game, action)

	// 检查是否需要进入下一阶段
	checkAndUpdatePhase(game)

	return nil
}

// 生成角色列表
func generateRoles(playerCount int, mode models.GameMode) []models.Role {
	roles := make([]models.Role, 0)

	// 基础角色分配
	fmt.Printf("开始生成角色列表，玩家数量: %d, 游戏模式: %s\n", playerCount, mode)
	switch mode {
	case models.ClassicMode:
		// 经典模式：狼人2个，预言家1个，女巫1个，其余为村民
		roles = append(roles, models.Werewolf, models.Werewolf)
		roles = append(roles, models.Seer)
		roles = append(roles, models.Witch)
		fmt.Println("经典模式角色分配：2个狼人，1个预言家，1个女巫")

	case models.StandardMode:
		// 标准模式：增加猎人和守卫
		roles = append(roles, models.Werewolf, models.Werewolf)
		roles = append(roles, models.Seer)
		roles = append(roles, models.Witch)
		roles = append(roles, models.Hunter)
		roles = append(roles, models.Guard)
		fmt.Println("标准模式角色分配：2个狼人，1个预言家，1个女巫，1个猎人，1个守卫")

	case models.ExtendedMode:
		// 扩展模式：增加白狼王和丘比特
		roles = append(roles, models.Werewolf, models.WhiteWolf)
		roles = append(roles, models.Seer)
		roles = append(roles, models.Witch)
		roles = append(roles, models.Hunter)
		roles = append(roles, models.Guard)
		roles = append(roles, models.Cupid)
		fmt.Println("扩展模式角色分配：1个狼人，1个白狼王，1个预言家，1个女巫，1个猎人，1个守卫，1个丘比特")
	}

	// 补充村民角色
	villagerCount := playerCount - len(roles)
	for i := 0; i < villagerCount; i++ {
		roles = append(roles, models.Villager)
	}
	fmt.Printf("补充村民数量: %d\n", villagerCount)

	return roles
}

// 分配角色
func assignRoles(game *GameState) {
	fmt.Printf("开始分配角色，房间ID: %s, 玩家数量: %d\n", game.Room.ID, len(game.Players))
	playerCount := len(game.Players)
	roles := generateRoles(playerCount, game.Room.Mode)

	// 随机打乱角色顺序
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(roles), func(i, j int) {
		roles[i], roles[j] = roles[j], roles[i]
	})
	fmt.Println("角色顺序已随机打乱")

	// 分配角色给玩家
	for i := range game.Players {
		game.Players[i].Role = roles[i]
		game.Players[i].Alive = true
		fmt.Printf("玩家 %s (%s) 被分配角色: %s\n", game.Players[i].Name, game.Players[i].ID, roles[i])
	}
	fmt.Println("角色分配完成")
}

// 获取可用动作
func getAvailableActions(game *GameState) []string {
	actions := make([]string, 0)

	switch game.Phase {
	case PhaseNight:
		// 夜晚阶段的动作
		for _, player := range game.Players {
			if !player.Alive {
				continue
			}

			switch player.Role {
			case models.Werewolf, models.WhiteWolf:
				actions = append(actions, "kill")
			case models.Seer:
				actions = append(actions, "check")
			case models.Witch:
				actions = append(actions, "save", "poison")
			case models.Guard:
				actions = append(actions, "protect")
			}
		}

	case PhaseDay:
		// 白天阶段的动作
		actions = append(actions, "discuss")

	case PhaseVote:
		// 投票阶段的动作
		actions = append(actions, "vote")
	}

	return actions
}

// 验证动作是否有效
func isValidAction(game *GameState, action models.GameAction) bool {
	// 检查玩家是否存活
	var player models.Player
	for _, p := range game.Players {
		if p.ID == action.PlayerID {
			player = p
			break
		}
	}

	if !player.Alive {
		return false
	}

	// 根据游戏阶段和角色验证动作
	switch game.Phase {
	case PhaseNight:
		switch action.Type {
		case "kill":
			return player.Role == models.Werewolf || player.Role == models.WhiteWolf
		case "check":
			return player.Role == models.Seer
		case "save", "poison":
			return player.Role == models.Witch
		case "protect":
			return player.Role == models.Guard
		default:
			return false
		}

	case PhaseDay:
		return action.Type == "discuss"

	case PhaseVote:
		return action.Type == "vote"

	default:
		return false
	}
}

// 处理动作结果
func processActionResult(game *GameState, action models.GameAction) {
	switch action.Type {
	case "kill":
		// 处理狼人杀人
		for i := range game.Players {
			if game.Players[i].ID == action.TargetID {
				game.Players[i].Alive = false
				break
			}
		}

	case "save", "poison":
		// 女巫救人或毒人
		for i := range game.Players {
			if game.Players[i].ID == action.TargetID {
				if action.Type == "save" {
					game.Players[i].Alive = true
				} else {
					game.Players[i].Alive = false
				}
				break
			}
		}

	case "vote":
		// 处理投票结果
		for i := range game.Players {
			if game.Players[i].ID == action.TargetID {
				game.Players[i].Alive = false
				break
			}
		}
	}
}

// 检查和更新游戏阶段
func checkAndUpdatePhase(game *GameState) {
	// 检查是否所有玩家都完成了当前阶段的动作
	switch game.Phase {
	case PhaseNight:
		// 夜晚结束，进入白天
		game.Phase = PhaseDay

	case PhaseDay:
		// 白天讨论结束，进入投票
		game.Phase = PhaseVote

	case PhaseVote:
		// 投票结束，进入新的夜晚，回合数加1
		game.Phase = PhaseNight
		game.Round++
	}

	// 重置阶段时间
	game.TimeLeft = 120 // 2分钟
}
