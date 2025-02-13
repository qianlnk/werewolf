package services

import (
	"errors"
	"sync"
	"time"

	"github.com/qianlnk/werewolf/models"
)

// GameState 游戏状态
type GameState struct {
	RoomID    string                  `json:"room_id"`
	Room      models.Room             `json:"room"`
	Players   []models.Player         `json:"players"`
	Phase     string                  `json:"phase"`
	Round     int                     `json:"round"`
	Actions   []models.GameAction     `json:"actions"`
	TimeLeft  int                     `json:"time_left"`
	IsStarted bool                    `json:"is_started"`
	Skills    map[string]*WitchSkills `json:"skills"` // 玩家技能状态
	mutex     sync.RWMutex
}

// NewGameState 创建游戏状态实例
func NewGameState(room models.Room) *GameState {
	return &GameState{
		Room:      room,
		Players:   room.Players,
		Phase:     PhaseNight,
		Round:     1,
		Actions:   make([]models.GameAction, 0),
		TimeLeft:  120, // 每个阶段默认120秒
		IsStarted: false,
		Skills:    make(map[string]*WitchSkills),
	}
}

// StartGame 开始游戏
func (gs *GameState) StartGame() error {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if len(gs.Players) < gs.Room.MinPlayers {
		return errors.New("玩家人数不足")
	}

	// 分配角色
	assignRoles(gs)

	// 初始化技能状态
	gs.initializeSkills()

	// 初始化游戏状态
	gs.Phase = PhaseNight
	gs.Round = 1
	gs.TimeLeft = 120
	gs.IsStarted = true
	gs.Actions = make([]models.GameAction, 0)

	return nil
}

// AddAction 添加游戏动作
func (gs *GameState) AddAction(action models.GameAction) error {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if !gs.IsStarted {
		return ErrGameNotStarted
	}

	// 验证动作是否有效
	if !isValidAction(gs, action) {
		return errors.New("无效的动作")
	}

	// 验证目标玩家是否可以被选择
	if action.TargetID != "" {
		targetValid := false
		for _, player := range gs.Players {
			if player.ID == action.TargetID && player.Alive {
				// 检查是否可以对该玩家执行动作
				switch gs.Phase {
				case PhaseNight:
					// 夜晚阶段，狼人不能杀死其他狼人
					if action.Type == "kill" {
						if player.Role != models.Werewolf && player.Role != models.WhiteWolf {
							targetValid = true
						}
					} else {
						targetValid = true
					}
				case PhaseVote:
					// 投票阶段，所有存活玩家都可以被投票
					targetValid = true
				}
				break
			}
		}

		if !targetValid {
			return errors.New("无效的目标玩家")
		}
	}

	// 添加时间戳
	action.Timestamp = time.Now().Unix()
	gs.Actions = append(gs.Actions, action)

	return nil
}

// GetPlayerStatus 获取玩家状态
func (gs *GameState) GetPlayerStatus(playerID string) (*models.Player, error) {
	gs.mutex.RLock()
	defer gs.mutex.RUnlock()

	for _, player := range gs.Players {
		if player.ID == playerID {
			return &player, nil
		}
	}

	return nil, errors.New("玩家不存在")
}

// UpdateTimeLeft 更新剩余时间
func (gs *GameState) UpdateTimeLeft(seconds int) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	gs.TimeLeft = seconds
}

// GetAvailableActions 获取玩家可用动作
func (gs *GameState) GetAvailableActions(playerID string) []string {
	gs.mutex.RLock()
	defer gs.mutex.RUnlock()

	var player *models.Player
	for i := range gs.Players {
		if gs.Players[i].ID == playerID {
			player = &gs.Players[i]
			break
		}
	}

	if player == nil || !player.Alive {
		return nil
	}

	return getAvailableActions(gs)
}

// initializeSkills 初始化玩家技能状态
func (gs *GameState) initializeSkills() {
	for _, player := range gs.Players {
		if player.Role == models.Witch {
			gs.Skills[player.ID] = &WitchSkills{
				SavePotion:   SkillStatus{Used: false},
				PoisonPotion: SkillStatus{Used: false},
			}
		}
	}
}
