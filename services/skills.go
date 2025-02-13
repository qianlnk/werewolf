package services

import (
	"errors"

	"github.com/qianlnk/werewolf/models"
)

// SkillManager 技能管理器
type SkillManager struct {
	game *GameState
}

// NewSkillManager 创建技能管理器实例
func NewSkillManager(game *GameState) *SkillManager {
	return &SkillManager{game: game}
}

// 技能使用状态
type SkillStatus struct {
	Used   bool
	Target string
}

// 女巫技能状态
type WitchSkills struct {
	SavePotion   SkillStatus
	PoisonPotion SkillStatus
}

// 使用预言家技能
func (sm *SkillManager) UseSeerSkill(seerID string, targetID string) (models.Role, error) {
	// 验证预言家身份
	seer := sm.findPlayer(seerID)
	if seer == nil || seer.Role != models.Seer {
		return "", errors.New("非预言家角色")
	}

	// 验证目标玩家
	target := sm.findPlayer(targetID)
	if target == nil {
		return "", errors.New("目标玩家不存在")
	}

	// 记录查验动作
	sm.game.Actions = append(sm.game.Actions, models.GameAction{
		Type:     "check",
		PlayerID: seerID,
		TargetID: targetID,
	})

	return target.Role, nil
}

// 使用女巫技能
func (sm *SkillManager) UseWitchSkill(witchID string, targetID string, skillType string) error {
	// 验证女巫身份
	witch := sm.findPlayer(witchID)
	if witch == nil || witch.Role != models.Witch {
		return errors.New("非女巫角色")
	}

	// 验证目标玩家
	target := sm.findPlayer(targetID)
	if target == nil {
		return errors.New("目标玩家不存在")
	}

	// 检查技能是否可用
	skills := sm.getWitchSkills(witchID)
	switch skillType {
	case "save":
		if skills.SavePotion.Used {
			return errors.New("救人技能已使用")
		}
		skills.SavePotion.Used = true
		skills.SavePotion.Target = targetID
	case "poison":
		if skills.PoisonPotion.Used {
			return errors.New("毒药已使用")
		}
		skills.PoisonPotion.Used = true
		skills.PoisonPotion.Target = targetID
	default:
		return errors.New("无效的技能类型")
	}

	// 记录技能使用
	sm.game.Actions = append(sm.game.Actions, models.GameAction{
		Type:     skillType,
		PlayerID: witchID,
		TargetID: targetID,
	})

	return nil
}

// 使用猎人技能
func (sm *SkillManager) UseHunterSkill(hunterID string, targetID string) error {
	// 验证猎人身份
	hunter := sm.findPlayer(hunterID)
	if hunter == nil || hunter.Role != models.Hunter {
		return errors.New("非猎人角色")
	}

	// 验证目标玩家
	target := sm.findPlayer(targetID)
	if target == nil {
		return errors.New("目标玩家不存在")
	}

	// 记录技能使用
	sm.game.Actions = append(sm.game.Actions, models.GameAction{
		Type:     "shoot",
		PlayerID: hunterID,
		TargetID: targetID,
	})

	// 处理猎人技能效果
	target.Alive = false

	return nil
}

// 使用守卫技能
func (sm *SkillManager) UseGuardSkill(guardID string, targetID string) error {
	// 验证守卫身份
	guard := sm.findPlayer(guardID)
	if guard == nil || guard.Role != models.Guard {
		return errors.New("非守卫角色")
	}

	// 验证目标玩家
	target := sm.findPlayer(targetID)
	if target == nil {
		return errors.New("目标玩家不存在")
	}

	// 记录技能使用
	sm.game.Actions = append(sm.game.Actions, models.GameAction{
		Type:     "protect",
		PlayerID: guardID,
		TargetID: targetID,
	})

	return nil
}

// 辅助函数：查找玩家
func (sm *SkillManager) findPlayer(playerID string) *models.Player {
	for i := range sm.game.Players {
		if sm.game.Players[i].ID == playerID {
			return &sm.game.Players[i]
		}
	}
	return nil
}

// 辅助函数：获取女巫技能状态
func (sm *SkillManager) getWitchSkills(witchID string) *WitchSkills {
	// 在实际实现中，这里应该从游戏状态中获取或初始化女巫技能状态
	// 这里简单返回一个新的实例
	return &WitchSkills{}
}
