// WebSocket连接实例
let gameWs = null;
let currentRoom = null;
let currentPlayer = null;
let isConnecting = false;
let reconnectAttempts = 0;
let connectionId = null;
const maxReconnectAttempts = 5;
const reconnectDelay = 3000;

// 初始化游戏房间
// 保存房间和玩家状态到本地存储
function saveGameState() {
    const gameState = {
        room: currentRoom,
        player: currentPlayer,
        connectionId: connectionId,
        timestamp: Date.now()
    };
    localStorage.setItem('gameState', JSON.stringify(gameState));
}

// 从本地存储恢复游戏状态
function restoreGameState() {
    const savedState = localStorage.getItem('gameState');
    if (savedState) {
        const gameState = JSON.parse(savedState);
        // 检查状态是否在24小时内
        if (Date.now() - gameState.timestamp < 24 * 60 * 60 * 1000) {
            return gameState;
        } else {
            localStorage.removeItem('gameState');
        }
    }
    return null;
}

// 修改initGameRoom函数
    // 修改initGameRoom函数
    function initGameRoom() {
        // 尝试从URL获取参数
        const urlParams = new URLSearchParams(window.location.search);
        let roomId = urlParams.get('room');
        let playerId = urlParams.get('player');
        
        // 如果URL中没有参数，尝试从本地存储恢复
        if (!roomId || !playerId) {
            const savedState = restoreGameState();
            if (savedState) {
                currentRoom = savedState.room;
                currentPlayer = savedState.player;
                // 更新URL，但不刷新页面
                const newUrl = `${window.location.pathname}?room=${currentRoom}&player=${currentPlayer.id}`;
                window.history.pushState({ path: newUrl }, '', newUrl);
            } else {
                $.messager.alert('错误', '缺少必要的房间或玩家信息');
                window.location.href = '/';
                return;
            }
        } else {
            currentRoom = roomId;
            currentPlayer = {
                id: playerId,
                name: localStorage.getItem('playerName')
            };
        }

        // 保存当前状态
        saveGameState();
        
        // 直接初始化WebSocket连接，不再通过API验证
        updateRoomInfo();
        initGameWebSocket();
        bindGameEvents();
    }


// 初始化游戏WebSocket连接
function initGameWebSocket() {
    if (isConnecting || (gameWs && gameWs.readyState === WebSocket.OPEN)) {
        return;
    }

    isConnecting = true;
    // 如果没有connectionId，生成一个新的
    if (!connectionId) {
        connectionId = generateConnectionId();
    }
    
    const wsUrl = `ws://${window.location.host}/ws?room=${currentRoom}&player=${currentPlayer.id}&connection_id=${connectionId}`;
    
    try {
        gameWs = new WebSocket(wsUrl);
        
        gameWs.onopen = function() {
            console.log('游戏房间WebSocket连接已建立');
            isConnecting = false;
            reconnectAttempts = 0;
            // 保存包含connectionId的状态
            saveGameState();
            // 连接成功后更新房间信息
            updateRoomInfo();
        };
        
        gameWs.onmessage = function(event) {
            try {
                if (!event.data) {
                    console.warn('收到空消息');
                    return;
                }
                const message = JSON.parse(event.data);
                if (!message || !message.type) {
                    console.warn('消息格式不正确:', message);
                    return;
                }
                handleGameMessage(message);
            } catch (error) {
                console.error('解析WebSocket消息失败:', error);
            }
        };
        
        gameWs.onclose = function(event) {
            console.log('游戏房间WebSocket连接已关闭, 代码:', event.code, '原因:', event.reason);
            isConnecting = false;
            gameWs = null;

            if (document.visibilityState === 'visible' && reconnectAttempts < maxReconnectAttempts) {
                reconnectAttempts++;
                const delay = Math.min(reconnectDelay * Math.pow(2, reconnectAttempts - 1), 30000);
                console.log(`尝试重新连接 (${reconnectAttempts}/${maxReconnectAttempts}), 等待${delay/1000}秒...`);
                setTimeout(initGameWebSocket, delay);
            } else if (reconnectAttempts >= maxReconnectAttempts) {
                console.error('WebSocket重连次数超过最大限制');
                $.messager.alert('错误', '网络连接失败，请刷新页面重试');
            }
        };
        
        gameWs.onerror = function(error) {
            console.error('游戏房间WebSocket错误:', error);
            isConnecting = false;
        };
    } catch (error) {
        console.error('初始化WebSocket失败:', error);
        isConnecting = false;
    }
}

// 生成连接ID
function generateConnectionId() {
    return 'conn_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
}

// 处理游戏消息
function handleGameMessage(message) {
    console.log('收到消息:', message);
    
    try {
        if (!message || !message.type) {
            console.error('无效的消息格式:', message);
            return;
        }
        
        // 直接处理room_update类型的消息
        if (message.type === 'room_update' && message.players) {
            updatePlayerList(message.players);
            return;
        }
        
        switch(message.type) {
            case 'broadcast':
                handleBroadcastMessage(message.content);
                break;
            case 'private':
                handlePrivateMessage(message.content);
                break;
            case 'error':
                $.messager.alert('错误', message.content.message);
                break;
            case 'chat':
                appendChatMessage(message);
                break;
            default:
                console.warn('未知的消息类型:', message.type);
        }
    } catch (error) {
        console.error('处理消息时发生错误:', error);
    }
}

// 处理广播消息
function handleBroadcastMessage(content) {
    switch(content.type) {
        case 'game_state':
            updateGameState(content);
            break;
        case 'chat':
            appendChatMessage(content);
            break;
        case 'room_update':
        case 'player_left':
            if (content.players) {
                updatePlayerList(content.players);
            }
            break;
    }
}

// 处理私人消息
function handlePrivateMessage(content) {
    switch(content.type) {
        case 'role_assign':
            updatePlayerRole(content);
            break;
    }
}

// 更新房间信息
function updateRoomInfo() {
    fetch(`/api/rooms/${currentRoom}`)
        .then(response => response.json())
        .then(room => {
            $('#roomName').text(room.name);
            updatePlayerList(room.players);
        })
        .catch(error => {
            console.error('获取房间信息失败:', error);
            $.messager.alert('错误', '获取房间信息失败: ' + error.message);
        });
}

// 更新玩家列表
function updatePlayerList(players) {
    const container = $('#playerContainer');
    container.empty();
    
    players.forEach(player => {
        const isCurrentPlayer = player.id === currentPlayer.id;
        const playerCard = $('<div>')
            .addClass('player-card')
            .addClass(player.alive === false ? 'dead' : 'alive')
            .addClass(isCurrentPlayer ? 'current-player' : '')
            .html(`
                <div class="player-name">${player.name}${isCurrentPlayer ? ' (你)' : ''}</div>
                ${player.role && (isCurrentPlayer || !player.alive) ? `<div class="player-role">角色: ${player.role}</div>` : ''}
            `);
        container.append(playerCard);
    });
}

// 更新游戏状态
function updateGameState(state) {
    $('#gameStatus').text(state.phase || '等待开始');
    if (state.time_left) {
        $('#gameTimer').text(`剩余时间: ${state.time_left}秒`);
    }
    if (state.players) {
        updatePlayerList(state.players);
    }
}

// 添加聊天消息
function appendChatMessage(chat) {
    const chatBox = $('#chatBox');
    const messageDiv = $('<div>')
        .addClass('chat-message')
        .html(`<span class="chat-player">${chat.player_id === currentPlayer.id ? '你' : chat.player_id}:</span> ${chat.message}`);
    chatBox.append(messageDiv);
    chatBox.scrollTop(chatBox[0].scrollHeight);
}

// 更新玩家角色信息
function updatePlayerRole(roleInfo) {
    $('#playerRole').show();
    $('#roleName').text(roleInfo.name);
    $('#roleDescription').text(roleInfo.description);
}

// 发送聊天消息
function sendMessage() {
    const input = $('#chatInput');
    const message = input.val().trim();
    
    if (message && gameWs && gameWs.readyState === WebSocket.OPEN) {
        const chatMessage = {
            type: 'chat',
            room_id: currentRoom,
            content: {
                message: message
            }
        };
        
        gameWs.send(JSON.stringify(chatMessage));
        input.val('');
    }
}

// 开始游戏
function startGame() {
    if (gameWs && gameWs.readyState === WebSocket.OPEN) {
        const startGameMessage = {
            type: 'game_action',
            room_id: currentRoom,
            content: {
                type: 'start_game'
            }
        };
        
        gameWs.send(JSON.stringify(startGameMessage));
    }
}

// 离开房间
function leaveRoom() {
    if (gameWs) {
        gameWs.close();
    }
    window.location.href = '/';
}

// 绑定游戏事件
function bindGameEvents() {
    $('#chatInput').keypress(function(e) {
        if (e.which == 13) {
            sendMessage();
        }
    });
}

// 页面加载完成后初始化
$(document).ready(function() {
    initGameRoom();
});

// 发送游戏动作
function sendGameAction(actionType, targetId) {
    if (!gameWs || gameWs.readyState !== WebSocket.OPEN) {
        $.messager.alert('错误', '网络连接已断开');
        return;
    }

    if (!currentRoom) {
        $.messager.alert('错误', '房间信息不存在');
        return;
    }

    if (!actionType) {
        $.messager.alert('错误', '无效的动作类型');
        return;
    }

    if (!targetId) {
        $.messager.alert('错误', '请选择目标玩家');
        return;
    }

    const gameAction = {
        type: 'game_action',
        room_id: currentRoom,
        content: {
            type: actionType,
            target: targetId
        }
    };

    gameWs.send(JSON.stringify(gameAction));
}