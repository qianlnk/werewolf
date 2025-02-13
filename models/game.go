package models

// GameMode 游戏模式
type GameMode string

const (
	ClassicMode  GameMode = "classic"  // 经典模式
	StandardMode GameMode = "standard" // 标准模式
	ExtendedMode GameMode = "extended" // 扩展模式
)

// Role 游戏角色
type Role string

const (
	// 基础角色
	Werewolf Role = "werewolf" // 狼人
	Seer     Role = "seer"     // 预言家
	Witch    Role = "witch"    // 女巫
	Villager Role = "villager" // 村民

	// 标准模式角色
	Hunter Role = "hunter" // 猎人
	Guard  Role = "guard"  // 守卫

	// 扩展模式角色
	Cupid     Role = "cupid"     // 丘比特
	Thief     Role = "thief"     // 盗贼
	WhiteWolf Role = "whitewolf" // 白狼王
)

// PlayerType 玩家类型
type PlayerType string

const (
	HumanPlayer PlayerType = "human" // 真人玩家
	AIPlayer    PlayerType = "ai"    // AI玩家
)

// AIPersonality AI性格特征
type AIPersonality string

const (
	Aggressive AIPersonality = "aggressive" // 激进型
	Analytical AIPersonality = "analytical" // 分析型
	Deceptive  AIPersonality = "deceptive"  // 伪装型
)

// Player 玩家信息
type Player struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        PlayerType    `json:"type"`
	Role        Role          `json:"role"`
	Personality AIPersonality `json:"personality,omitempty"`
	Alive       bool          `json:"alive"`
	IsLover     bool          `json:"is_lover"` // 是否是情侣
}

// Room 游戏房间
type Room struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Mode        GameMode `json:"mode"`
	Players     []Player `json:"players"`
	MaxPlayers  int      `json:"max_players"`
	MinPlayers  int      `json:"min_players"`
	GameStarted bool     `json:"game_started"`
	CreatedAt   int64    `json:"created_at"`
}

// GameAction 游戏动作
type GameAction struct {
	Type      string `json:"type"`
	PlayerID  string `json:"player_id"`
	TargetID  string `json:"target_id,omitempty"`
	Timestamp int64  `json:"timestamp"`
	RoomID    string `json:"room_id"`           // 房间ID
	Content   string `json:"content,omitempty"` // 动作内容
}

// GameStatus 游戏状态
type GameStatus struct {
	Phase    string   `json:"phase"`     // day, night
	Round    int      `json:"round"`     // 游戏轮次
	Players  []Player `json:"players"`   // 玩家列表
	Actions  []string `json:"actions"`   // 可执行的动作
	TimeLeft int      `json:"time_left"` // 剩余时间
}
