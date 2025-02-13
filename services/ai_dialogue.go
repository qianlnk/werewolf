package services

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/qianlnk/werewolf/models"
)

// AIDialogue AI对话生成器
type AIDialogue struct {
	game *GameState
}

// NewAIDialogue 创建AI对话生成器实例
func NewAIDialogue(game *GameState) *AIDialogue {
	return &AIDialogue{game: game}
}

// GenerateDialogue 生成AI对话内容
func (ad *AIDialogue) GenerateDialogue(player models.Player) string {
	switch ad.game.Phase {
	case PhaseDay:
		return ad.generateDayDialogue(player)
	case PhaseVote:
		return ad.generateVoteDialogue(player)
	default:
		return ""
	}
}

// generateDayDialogue 生成白天阶段的对话
func (ad *AIDialogue) generateDayDialogue(player models.Player) string {
	// 根据角色和性格生成对话
	switch player.Role {
	case models.Werewolf, models.WhiteWolf:
		return ad.generateWerewolfDayDialogue(player)
	case models.Villager:
		return ad.generateVillagerDayDialogue(player)
	case models.Seer:
		return ad.generateSeerDayDialogue(player)
	default:
		return ad.generateDefaultDayDialogue(player)
	}
}

// generateVoteDialogue 生成投票阶段的对话
func (ad *AIDialogue) generateVoteDialogue(player models.Player) string {
	// 分析游戏局势
	suspects := ad.analyzeSuspects(player)
	if len(suspects) > 0 {
		target := suspects[rand.Intn(len(suspects))]
		return fmt.Sprintf("我认为%s比较可疑，建议大家投票给ta", target.Name)
	}
	return "这局形势不太明朗，大家要谨慎投票"
}

// generateWerewolfDayDialogue 生成狼人白天对话
func (ad *AIDialogue) generateWerewolfDayDialogue(player models.Player) string {
	// 狼人需要伪装和误导
	responses := []string{
		"昨晚我好像听到了一些动静，但不确定是什么",
		"我觉得我们要相信预言家，但也要防止有人冒充",
		"大家要冷静分析，不要被表象迷惑",
	}
	return responses[rand.Intn(len(responses))]
}

// generateVillagerDayDialogue 生成村民白天对话
func (ad *AIDialogue) generateVillagerDayDialogue(player models.Player) string {
	// 村民需要积极找出狼人
	responses := []string{
		"大家有没有发现什么可疑的人？",
		"我们要抓紧时间找出狼人",
		"昨晚的情况大家怎么看？",
	}
	return responses[rand.Intn(len(responses))]
}

// generateSeerDayDialogue 生成预言家白天对话
func (ad *AIDialogue) generateSeerDayDialogue(player models.Player) string {
	// 预言家需要引导方向
	responses := []string{
		"我有一些重要的信息要分享",
		"大家要相信我的判断",
		"我觉得有些人的行为很值得怀疑",
	}
	return responses[rand.Intn(len(responses))]
}

// generateDefaultDayDialogue 生成默认白天对话
func (ad *AIDialogue) generateDefaultDayDialogue(player models.Player) string {
	responses := []string{
		"让我们好好分析一下局势",
		"大家有什么想法吗？",
		"我们要团结一致找出狼人",
	}
	return responses[rand.Intn(len(responses))]
}

// analyzeSuspects 分析可疑玩家
func (ad *AIDialogue) analyzeSuspects(player models.Player) []models.Player {
	suspects := make([]models.Player, 0)
	for _, p := range ad.game.Players {
		if !p.Alive || p.ID == player.ID {
			continue
		}

		// 分析玩家行为和发言
		if isSuspicious(p, ad.game.Actions) {
			suspects = append(suspects, p)
		}
	}
	return suspects
}

// isSuspicious 判断玩家是否可疑
func isSuspicious(player models.Player, actions []models.GameAction) bool {
	// 实现可疑行为判断逻辑
	// 例如：分析投票模式、发言矛盾等
	// 这里使用随机值作为示例
	rand.Seed(time.Now().UnixNano())
	return rand.Float64() < 0.3 // 30%的概率判定为可疑
}
