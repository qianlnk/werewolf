# AI狼人杀游戏系统

## 项目简介
这是一个基于Golang和Easy-UI开发的AI狼人杀游戏系统。系统支持真人玩家与AI玩家混合对战，提供多种游戏模式和角色配置。

## 技术栈
- 后端：Golang
  - WebSocket实时通信
  - RESTful API
  - 状态机管理
- 前端：Easy-UI
  - 响应式界面
  - WebSocket客户端
  - 动态交互组件

## 目录结构
```
/backend         # 后端Go代码
  /api          # API接口定义
  /config       # 配置文件
  /models       # 数据模型
  /services     # 业务逻辑
  /utils        # 工具函数
  /websocket    # WebSocket处理

/frontend       # 前端Easy-UI代码
  /css          # 样式文件
  /js           # JavaScript文件
  /pages        # 页面模板
  /components   # UI组件

/docs           # 项目文档
```

## 功能特点
- 支持5-12人的多种游戏模式
- AI玩家具有不同性格特征和发言风格
- 实时语音/文字交互
- 角色动态分配
- 多种胜负判定机制
- 游戏数据分析和复盘

## 安装和运行
1. 克隆项目
```bash
git clone https://github.com/qianlnk/werewolf.git
```

2. 安装依赖
```bash
# 后端依赖
go mod download

# 前端依赖
cd frontend
npm install
```

3. 运行项目
```bash
# 启动后端服务
go run main.go

# 启动前端服务
cd frontend
npm start
```

## 开发进度
- [x] 项目基础框架搭建
- [ ] 后端API实现
- [ ] WebSocket通信
- [ ] 前端界面开发
- [ ] AI决策系统
- [ ] 游戏逻辑实现
- [ ] 测试和优化

## 贡献指南
欢迎提交Issue和Pull Request。

## 许可证
MIT License