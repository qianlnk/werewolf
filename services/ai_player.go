package services

import (
	"math/rand"
	"time"

	"github.com/qianlnk/werewolf/models"
)

// 性格特征
const (
	PersonalityAggressive = "aggressive" // 激进
	PersonalityCautious   = "cautious"   // 谨慎
	PersonalityStrategic  = "strategic"  // 策略
	PersonalityRandom     = "random"     // 随机
)

// AIPlayer AI玩家
type AIPlayer struct {
	ID           string
	Personality  string
	Role         models.Role
	GameState    *GameState
	KnownPlayers map[string]models.Role // 已知的玩家角色信息
}

// NewAIPlayer 创建AI玩家实例
func NewAIPlayer(id string, role models.Role, gameState *GameState) *AIPlayer {
	personalities := []string{
		PersonalityAggressive,
		PersonalityCautious,
		PersonalityStrategic,
		PersonalityRandom,
	}

	rand.Seed(time.Now().UnixNano())
	return &AIPlayer{
		ID:           id,
		Personality:  personalities[rand.Intn(len(personalities))],
		Role:         role,
		GameState:    gameState,
		KnownPlayers: make(map[string]models.Role),
	}
}

// DecideAction 决定下一步行动
func (ai *AIPlayer) DecideAction() models.GameAction {
	switch ai.GameState.Phase {
	case PhaseNight:
		return ai.decideNightAction()
	case PhaseDay:
		return ai.decideDayAction()
	case PhaseVote:
		return ai.decideVoteAction()
	default:
		return models.GameAction{}
	}
}

// decideNightAction 决定夜晚行动
func (ai *AIPlayer) decideNightAction() models.GameAction {
	action := models.GameAction{
		PlayerID: ai.ID,
	}

	switch ai.Role {
	case models.Werewolf, models.WhiteWolf:
		action.Type = "kill"
		action.TargetID = ai.selectKillTarget()

	case models.Seer:
		action.Type = "check"
		action.TargetID = ai.selectCheckTarget()

	case models.Witch:
		action = ai.decideWitchAction()

	case models.Guard:
		action.Type = "protect"
		action.TargetID = ai.selectProtectTarget()
	}

	return action
}

// decideDayAction 决定白天行动
func (ai *AIPlayer) decideDayAction() models.GameAction {
	return models.GameAction{
		PlayerID: ai.ID,
		Type:     "discuss",
		Content:  ai.generateDiscussion(),
	}
}

// decideVoteAction 决定投票行动
func (ai *AIPlayer) decideVoteAction() models.GameAction {
	return models.GameAction{
		PlayerID: ai.ID,
		Type:     "vote",
		TargetID: ai.selectVoteTarget(),
	}
}

// selectKillTarget 选择击杀目标
func (ai *AIPlayer) selectKillTarget() string {
	var potentialTargets []string

	for _, player := range ai.GameState.Players {
		if !player.Alive || player.Role == models.Werewolf || player.Role == models.WhiteWolf {
			continue
		}

		switch ai.Personality {
		case PersonalityAggressive:
			// 优先击杀特殊角色
			if role, known := ai.KnownPlayers[player.ID]; known &&
				(role == models.Seer || role == models.Witch) {
				return player.ID
			}

		case PersonalityCautious:
			// 优先击杀可能威胁较小的目标
			if _, known := ai.KnownPlayers[player.ID]; !known {
				potentialTargets = append(potentialTargets, player.ID)
			}

		case PersonalityStrategic:
			// 根据游戏局势选择目标
			if role, known := ai.KnownPlayers[player.ID]; known && role == models.Villager {
				potentialTargets = append(potentialTargets, player.ID)
			}

		case PersonalityRandom:
			potentialTargets = append(potentialTargets, player.ID)
		}
	}

	if len(potentialTargets) > 0 {
		return potentialTargets[rand.Intn(len(potentialTargets))]
	}

	// 如果没有找到合适的目标，随机选择一个存活的非狼人玩家
	for _, player := range ai.GameState.Players {
		if player.Alive && player.Role != models.Werewolf && player.Role != models.WhiteWolf {
			return player.ID
		}
	}

	return ""
}

// selectCheckTarget 选择查验目标
func (ai *AIPlayer) selectCheckTarget() string {
	var potentialTargets []string

	for _, player := range ai.GameState.Players {
		if !player.Alive || player.ID == ai.ID || ai.KnownPlayers[player.ID] != "" {
			continue
		}

		switch ai.Personality {
		case PersonalityAggressive:
			// 优先查验可疑的玩家
			if ai.isSuspicious(player.ID) {
				return player.ID
			}

		case PersonalityCautious:
			// 优先查验安静的玩家
			if !ai.isActive(player.ID) {
				potentialTargets = append(potentialTargets, player.ID)
			}

		default:
			potentialTargets = append(potentialTargets, player.ID)
		}
	}

	if len(potentialTargets) > 0 {
		return potentialTargets[rand.Intn(len(potentialTargets))]
	}

	return ""
}

// decideWitchAction 决定女巫行动
func (ai *AIPlayer) decideWitchAction() models.GameAction {
	action := models.GameAction{
		PlayerID: ai.ID,
	}

	// 获取昨晚被杀的玩家
	killedPlayer := ai.getLastKilledPlayer()

	switch ai.Personality {
	case PersonalityAggressive:
		// 激进型女巫倾向于使用毒药
		if ai.hasPoison() && ai.isSuspicious(killedPlayer) {
			action.Type = "poison"
			action.TargetID = ai.selectPoisonTarget()
		} else if ai.hasSavePotion() && ai.isImportantPlayer(killedPlayer) {
			action.Type = "save"
			action.TargetID = killedPlayer
		}

	case PersonalityCautious:
		// 谨慎型女巫优先考虑救人
		if ai.hasSavePotion() && ai.isImportantPlayer(killedPlayer) {
			action.Type = "save"
			action.TargetID = killedPlayer
		}

	case PersonalityStrategic:
		// 策略型女巫根据局势决定
		if ai.hasSavePotion() && ai.shouldSavePlayer(killedPlayer) {
			action.Type = "save"
			action.TargetID = killedPlayer
		} else if ai.hasPoison() && ai.shouldPoisonPlayer() {
			action.Type = "poison"
			action.TargetID = ai.selectPoisonTarget()
		}

	default:
		// 随机型女巫随机决定
		if rand.Float64() < 0.5 && ai.hasSavePotion() && killedPlayer != "" {
			action.Type = "save"
			action.TargetID = killedPlayer
		} else if ai.hasPoison() {
			action.Type = "poison"
			action.TargetID = ai.selectPoisonTarget()
		}
	}

	return action
}

// 辅助方法
func (ai *AIPlayer) getLastKilledPlayer() string {
	// 获取昨晚被狼人杀害的玩家
	for _, action := range ai.GameState.Actions {
		if action.Type == "kill" {
			return action.TargetID
		}
	}
	return ""
}

func (ai *AIPlayer) isImportantPlayer(playerID string) bool {
	// 判断是否是重要角色（预言家、女巫等）
	if role, known := ai.KnownPlayers[playerID]; known {
		return role == models.Seer || role == models.Witch || role == models.Guard
	}
	return false
}

func (ai *AIPlayer) shouldSavePlayer(playerID string) bool {
	// 策略性判断是否应该救人
	if !ai.isImportantPlayer(playerID) {
		return false
	}
	// 根据游戏局势判断
	return ai.GameState.Round <= 3 || ai.countAlivePlayers() <= 6
}

func (ai *AIPlayer) shouldPoisonPlayer() bool {
	// 策略性判断是否应该使用毒药
	return ai.GameState.Round > 2 && ai.countSuspiciousPlayers() > 0
}

func (ai *AIPlayer) selectPoisonTarget() string {
	// 选择使用毒药的目标
	var potentialTargets []string

	for _, player := range ai.GameState.Players {
		if !player.Alive || player.ID == ai.ID {
			continue
		}

		if ai.isSuspicious(player.ID) {
			potentialTargets = append(potentialTargets, player.ID)
		}
	}

	if len(potentialTargets) > 0 {
		return potentialTargets[rand.Intn(len(potentialTargets))]
	}
	return ""
}

func (ai *AIPlayer) countAlivePlayers() int {
	count := 0
	for _, player := range ai.GameState.Players {
		if player.Alive {
			count++
		}
	}
	return count
}

func (ai *AIPlayer) countSuspiciousPlayers() int {
	count := 0
	for _, player := range ai.GameState.Players {
		if player.Alive && ai.isSuspicious(player.ID) {
			count++
		}
	}
	return count
}

func (ai *AIPlayer) hasSavePotion() bool {
	// 检查女巫是否还有解药
	if skills, exists := ai.GameState.Skills[ai.ID]; exists {
		return !skills.SavePotion.Used
	}
	return false
}

func (ai *AIPlayer) hasPoison() bool {
	// 检查女巫是否还有毒药
	if skills, exists := ai.GameState.Skills[ai.ID]; exists {
		return !skills.PoisonPotion.Used
	}
	return false
}

// selectProtectTarget 选择守护目标
func (ai *AIPlayer) selectProtectTarget() string {
	var potentialTargets []string

	for _, player := range ai.GameState.Players {
		if !player.Alive || player.ID == ai.ID {
			continue
		}

		switch ai.Personality {
		case PersonalityStrategic:
			// 优先守护重要角色
			if role, known := ai.KnownPlayers[player.ID]; known &&
				(role == models.Seer || role == models.Witch) {
				return player.ID
			}

		default:
			potentialTargets = append(potentialTargets, player.ID)
		}
	}

	if len(potentialTargets) > 0 {
		return potentialTargets[rand.Intn(len(potentialTargets))]
	}

	return ""
}

// selectVoteTarget 选择投票目标
func (ai *AIPlayer) selectVoteTarget() string {
	var potentialTargets []string

	for _, player := range ai.GameState.Players {
		if !player.Alive || player.ID == ai.ID {
			continue
		}

		switch ai.Personality {
		case PersonalityAggressive:
			// 优先投票可疑的玩家
			if ai.isSuspicious(player.ID) {
				return player.ID
			}

		case PersonalityCautious:
			// 跟随大多数人的投票
			if ai.isPopularVoteTarget(player.ID) {
				return player.ID
			}

		default:
			potentialTargets = append(potentialTargets, player.ID)
		}
	}

	if len(potentialTargets) > 0 {
		return potentialTargets[rand.Intn(len(potentialTargets))]
	}

	return ""
}

// generateDiscussion 生成讨论内容
func (ai *AIPlayer) generateDiscussion() string {
	// 根据角色和性格生成对话内容
	switch ai.Role {
	case models.Werewolf, models.WhiteWolf:
		return ai.generateWerewolfDiscussion()
	case models.Seer:
		return ai.generateSeerDiscussion()
	case models.Witch:
		return ai.generateWitchDiscussion()
	case models.Guard:
		return ai.generateGuardDiscussion()
	default:
		return ai.generateVillagerDiscussion()
	}
}

// generateWerewolfDiscussion 生成狼人对话
func (ai *AIPlayer) generateWerewolfDiscussion() string {
	switch ai.Personality {
	case PersonalityAggressive:
		return "我觉得有人在伪装预言家，我们应该投他"
	case PersonalityCautious:
		return "大家要冷静分析，不要轻易相信任何人的发言"
	case PersonalityStrategic:
		return "我们应该先听听预言家的发言，再做判断"
	default:
		return "昨晚的情况大家怎么看？"
	}
}

// generateSeerDiscussion 生成预言家对话
func (ai *AIPlayer) generateSeerDiscussion() string {
	switch ai.Personality {
	case PersonalityAggressive:
		return "我是预言家，昨晚我查验了一个人，发现是狼人"
	case PersonalityCautious:
		return "作为预言家，我建议大家要谨慎行动"
	case PersonalityStrategic:
		return "我有重要信息要分享，但现在说可能为时过早"
	default:
		return "让我们一起分析一下目前的局势"
	}
}

// generateWitchDiscussion 生成女巫对话
func (ai *AIPlayer) generateWitchDiscussion() string {
	switch ai.Personality {
	case PersonalityAggressive:
		return "我知道一些重要的信息，但需要大家配合"
	case PersonalityCautious:
		return "我们要小心行事，不要轻易相信任何人"
	case PersonalityStrategic:
		return "让我们先听听大家的想法，再做决定"
	default:
		return "昨晚发生了什么有趣的事情吗？"
	}
}

// generateGuardDiscussion 生成守卫对话
func (ai *AIPlayer) generateGuardDiscussion() string {
	switch ai.Personality {
	case PersonalityAggressive:
		return "我们必须保护好重要的角色"
	case PersonalityCautious:
		return "大家要注意安全，狼人可能会有突然袭击"
	case PersonalityStrategic:
		return "我觉得我们应该制定一个保护策略"
	default:
		return "大家有什么需要帮助的吗？"
	}
}

// generateVillagerDiscussion 生成村民对话
func (ai *AIPlayer) generateVillagerDiscussion() string {
	switch ai.Personality {
	case PersonalityAggressive:
		return "我觉得有人行为很可疑，应该仔细观察"
	case PersonalityCautious:
		return "我们要相信预言家，但也要防止有人冒充"
	case PersonalityStrategic:
		return "让我们分析一下每个人的发言，找出线索"
	default:
		return "大家对昨晚的情况有什么看法？"
	}
}

// 辅助函数
func (ai *AIPlayer) isSuspicious(playerID string) bool {
	// 统计玩家的可疑行为
	suspiciousScore := 0

	// 检查玩家的投票历史
	for _, action := range ai.GameState.Actions {
		if action.Type == "vote" && action.PlayerID == playerID {
			// 如果投票给已知的好人，增加可疑度
			if role, known := ai.KnownPlayers[action.TargetID]; known && role != models.Werewolf && role != models.WhiteWolf {
				suspiciousScore++
			}
		}
	}

	// 检查发言活跃度
	if !ai.isActive(playerID) {
		suspiciousScore++
	}

	// 根据预言家的验人结果
	if ai.Role == models.Seer {
		if role, known := ai.KnownPlayers[playerID]; known && (role == models.Werewolf || role == models.WhiteWolf) {
			return true
		}
	}

	return suspiciousScore >= 2
}

func (ai *AIPlayer) isActive(playerID string) bool {
	// 统计玩家在白天阶段的发言次数
	speakCount := 0
	for _, action := range ai.GameState.Actions {
		if action.Type == "discuss" && action.PlayerID == playerID {
			speakCount++
		}
	}

	// 根据游戏轮数判断发言活跃度
	expectedSpeakCount := ai.GameState.Round * 2 // 每轮期望至少发言两次
	return speakCount >= expectedSpeakCount
}

func (ai *AIPlayer) isPopularVoteTarget(playerID string) bool {
	// 统计当前投票阶段该玩家收到的票数
	votes := 0
	for _, action := range ai.GameState.Actions {
		if action.Type == "vote" && action.TargetID == playerID {
			votes++
		}
	}

	// 计算存活玩家数量
	aliveCount := 0
	for _, player := range ai.GameState.Players {
		if player.Alive {
			aliveCount++
		}
	}

	// 如果收到超过1/3的票数，认为是热门投票目标
	return votes >= aliveCount/3
}
