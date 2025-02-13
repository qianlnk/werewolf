package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/qianlnk/werewolf/models"
	"github.com/qianlnk/werewolf/services"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // 允许所有跨域请求，生产环境中应该更严格
		},
	}

	roomManager  *services.RoomManager
	webSocketMgr *services.WebSocketManager
	gameManager  = services.NewGameManager()
)

func init() {
	// 设置日志格式，包含文件名和行号
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	webSocketMgr = services.NewWebSocketManager(nil)
	roomManager = services.NewRoomManager(webSocketMgr)
	webSocketMgr.SetRoomManager(roomManager)

	// 添加日志记录
	log.Printf("初始化完成: WebSocket管理器和房间管理器已配置")
}

func main() {
	r := gin.Default()

	// 设置跨域中间件
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 静态文件服务
	r.Static("/css", "./frontend/css")
	r.Static("/js", "./frontend/js")
	r.Static("/static", "./frontend/static")

	// 加载HTML模板
	r.LoadHTMLGlob("frontend/*.html")

	// 前端页面路由
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	r.GET("/game", func(c *gin.Context) {
		c.HTML(http.StatusOK, "game.html", nil)
	})

	// WebSocket连接处理
	r.GET("/ws", func(c *gin.Context) {
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("升级WebSocket连接失败: %v", err)
			return
		}

		// 获取房间ID、玩家ID和连接ID
		roomID := c.Query("room")
		playerID := c.Query("player")
		connectionID := c.Query("connection_id")

		if roomID == "" || playerID == "" || connectionID == "" {
			log.Printf("缺少必要的连接参数")
			ws.Close()
			return
		}

		// 注册WebSocket连接，传入连接ID
		webSocketMgr.RegisterConnection(playerID, ws, connectionID)
		webSocketMgr.JoinRoom(roomID, playerID)
	})

	// API路由组
	api := r.Group("/api")
	{
		// 游戏房间相关
		api.POST("/rooms", createRoom)
		api.GET("/rooms", listRooms)
		api.GET("/rooms/:id", getRoomInfo)
		api.POST("/rooms/:id/join", joinRoom)
		api.GET("/rooms/:id/players/:playerId", getPlayerInfo)

		// 游戏操作相关
		api.POST("/game/action", gameAction)
		api.GET("/game/status", getGameStatus)
	}

	// 启动服务器
	log.Println("服务器启动在 :8080 端口")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("服务器启动失败:", err)
	}
}

// API处理函数
func createRoom(c *gin.Context) {
	var req struct {
		Name       string          `json:"name" binding:"required"`
		Mode       models.GameMode `json:"mode" binding:"required"`
		MaxPlayers int             `json:"max_players" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	room := roomManager.CreateRoom(req.Name, req.Mode, req.MaxPlayers)
	c.JSON(http.StatusOK, room)
}

func listRooms(c *gin.Context) {
	rooms := roomManager.ListRooms()
	c.JSON(http.StatusOK, gin.H{"rooms": rooms})
}

// 获取房间中的玩家信息
func getPlayerInfo(c *gin.Context) {
	roomID := c.Param("id")
	playerID := c.Param("playerId")

	player, err := roomManager.GetPlayer(roomID, playerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, player)
}

func getRoomInfo(c *gin.Context) {
	roomID := c.Param("id")

	room, err := roomManager.GetRoom(roomID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, room)
}

func joinRoom(c *gin.Context) {
	roomID := c.Param("id")
	var player models.Player
	if err := c.ShouldBindJSON(&player); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := roomManager.JoinRoom(roomID, player); err != nil {
		statusCode := http.StatusInternalServerError
		if err == services.ErrRoomNotFound {
			statusCode = http.StatusNotFound
		} else if err == services.ErrRoomFull {
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "加入房间成功"})
}

func gameAction(c *gin.Context) {
	var action models.GameAction
	if err := c.ShouldBindJSON(&action); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取房间ID
	roomID := action.RoomID
	game, exists := roomManager.GetGameController(roomID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "游戏未找到"})
		return
	}

	// 处理游戏动作
	if err := game.ProcessAction(action); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "动作执行成功"})
}

func getGameStatus(c *gin.Context) {
	// TODO: 实现获取游戏状态逻辑
	c.JSON(http.StatusOK, gin.H{"status": "game status"})
}
