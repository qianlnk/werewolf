package services

import (
	"errors"

	"github.com/qianlnk/werewolf/models"
)

// 游戏胜负状态
const (
	GameOngoing  = "ongoing"
	WerewolfWin  = "werewolf_win"
	VillagerWin  = "villager_win"
	LoversWin    = "lovers_win"
	WhiteWolfWin = "white_wolf_win"
)

// StateMachine 游戏状态机
type StateMachine struct {
	game   *GameState
	status string // 游戏状态：ongoing, werewolf_win, villager_win
}

// NewStateMachine 创建状态机实例
func NewStateMachine(game *GameState) *StateMachine {
	return &StateMachine{game: game}
}

// TransitionPhase 转换游戏阶段
func (sm *StateMachine) TransitionPhase() error {
	if !sm.game.IsStarted {
		return ErrGameNotStarted
	}

	// 检查当前阶段是否所有必要动作都已完成
	if !sm.isPhaseComplete() {
		return errors.New("当前阶段尚未完成所有必要动作")
	}

	// 更新游戏阶段
	switch sm.game.Phase {
	case PhaseNight:
		// 处理夜晚阶段的结果
		sm.processNightResults()
		sm.game.Phase = PhaseDay

	case PhaseDay:
		// 白天阶段结束后进入投票
		sm.game.Phase = PhaseVote

	case PhaseVote:
		// 处理投票结果
		sm.processVoteResults()
		// 进入新的夜晚
		sm.game.Phase = PhaseNight
		sm.game.Round++
	}

	// 重置阶段时间
	sm.game.TimeLeft = 120

	// 检查游戏是否结束
	return sm.checkGameEnd()
}

// isPhaseComplete 检查当前阶段是否完成
func (sm *StateMachine) isPhaseComplete() bool {
	switch sm.game.Phase {
	case PhaseNight:
		return sm.checkNightActionsComplete()
	case PhaseDay:
		return sm.game.TimeLeft <= 0
	case PhaseVote:
		return sm.checkVoteComplete()
	default:
		return false
	}
}

// checkNightActionsComplete 检查夜晚行动是否完成
func (sm *StateMachine) checkNightActionsComplete() bool {
	// 检查每个活着的特殊角色是否完成了行动
	for _, player := range sm.game.Players {
		if !player.Alive {
			continue
		}

		switch player.Role {
		case models.Werewolf, models.WhiteWolf:
			if !sm.hasActionOfType(player.ID, "kill") {
				return false
			}
		case models.Seer:
			if !sm.hasActionOfType(player.ID, "check") {
				return false
			}
		case models.Witch:
			// 女巫可以选择不使用技能
			continue
		case models.Guard:
			if !sm.hasActionOfType(player.ID, "protect") {
				return false
			}
		}
	}
	return true
}

// checkVoteComplete 检查投票是否完成
func (sm *StateMachine) checkVoteComplete() bool {
	// 检查是否所有活着的玩家都已投票
	voteCount := 0
	aliveCount := 0
	for _, player := range sm.game.Players {
		if player.Alive {
			aliveCount++
			if sm.hasActionOfType(player.ID, "vote") {
				voteCount++
			}
		}
	}
	return voteCount == aliveCount
}

// hasActionOfType 检查玩家是否执行了特定类型的动作
func (sm *StateMachine) hasActionOfType(playerID, actionType string) bool {
	for _, action := range sm.game.Actions {
		if action.PlayerID == playerID && action.Type == actionType {
			return true
		}
	}
	return false
}

// processNightResults 处理夜晚阶段的结果
func (sm *StateMachine) processNightResults() {
	// 处理狼人击杀
	for _, action := range sm.game.Actions {
		if action.Type == "kill" {
			processActionResult(sm.game, action)
		}
	}

	// 处理女巫救人或毒人
	for _, action := range sm.game.Actions {
		if action.Type == "save" || action.Type == "poison" {
			processActionResult(sm.game, action)
		}
	}

	// 清空行动列表
	sm.game.Actions = make([]models.GameAction, 0)
}

// processVoteResults 处理投票结果
func (sm *StateMachine) processVoteResults() {
	// 统计票数
	votes := make(map[string]int)
	for _, action := range sm.game.Actions {
		if action.Type == "vote" {
			votes[action.TargetID]++
		}
	}

	// 找出票数最多的玩家
	maxVotes := 0
	var eliminatedID string
	for playerID, count := range votes {
		if count > maxVotes {
			maxVotes = count
			eliminatedID = playerID
		}
	}

	// 处理投票结果
	if eliminatedID != "" {
		action := models.GameAction{
			Type:     "vote",
			TargetID: eliminatedID,
		}
		processActionResult(sm.game, action)
	}

	// 清空行动列表
	sm.game.Actions = make([]models.GameAction, 0)
}

// checkGameEnd 检查游戏是否结束
func (sm *StateMachine) checkGameEnd() error {
	// 统计各阵营存活人数
	werewolfCount := 0
	villagerCount := 0
	whiteWolfCount := 0
	loversAlive := 0
	loversWolfCount := 0
	loversVillagerCount := 0

	// 统计存活人数
	for _, player := range sm.game.Players {
		if !player.Alive {
			continue
		}

		// 检查情侣存活状态
		if player.IsLover {
			loversAlive++
			// 统计情侣中的狼人和好人数量
			if player.Role == models.Werewolf || player.Role == models.WhiteWolf {
				loversWolfCount++
			} else {
				loversVillagerCount++
			}
		}

		// 统计不同阵营人数
		switch player.Role {
		case models.WhiteWolf:
			whiteWolfCount++
			werewolfCount++
		case models.Werewolf:
			werewolfCount++
		default:
			villagerCount++
		}
	}

	// 判定特殊胜利条件
	// 1. 情侣胜利：只剩下情侣存活
	if loversAlive == 2 && loversAlive == villagerCount+werewolfCount {
		sm.status = LoversWin
		return errors.New("情侣阵营胜利：只剩下情侣存活")
	}

	// 2. 白狼王觉醒胜利：只剩白狼王一人
	if whiteWolfCount == 1 && werewolfCount == 1 && villagerCount == 0 {
		sm.status = WhiteWolfWin
		return errors.New("白狼王觉醒胜利：白狼王成为最后的胜利者")
	}

	// 常规胜利条件判定
	if werewolfCount == 0 {
		sm.status = VillagerWin
		return errors.New("好人阵营胜利：所有狼人都已被清除")
	} else if werewolfCount >= villagerCount {
		sm.status = WerewolfWin
		return errors.New("狼人阵营胜利：狼人数量已经超过或等于好人数量")
	}

	sm.status = GameOngoing
	return nil
}
