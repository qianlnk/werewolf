// WebSocket连接
let ws = null;
let isConnecting = false;
let reconnectAttempts = 0;
const maxReconnectAttempts = 5;
const reconnectDelay = 3000;

// 初始化WebSocket连接
function initWebSocket() {
    if (isConnecting || (ws && ws.readyState === WebSocket.OPEN)) {
        return;
    }

    isConnecting = true;
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const urlParams = new URLSearchParams(window.location.search);
    const roomId = urlParams.get('room');
    const playerId = urlParams.get('player');
    const wsUrl = `${protocol}//${window.location.host}/ws?room=${roomId}&player=${playerId}`;
    
    try {
        ws = new WebSocket(wsUrl);
        
        // 设置连接超时
        const connectionTimeout = setTimeout(() => {
            if (ws.readyState !== WebSocket.OPEN) {
                console.error('WebSocket连接超时');
                ws.close();
                handleReconnect();
            }
        }, 5000);
        
        ws.onopen = function() {
            console.log('WebSocket连接已建立');
            clearTimeout(connectionTimeout);
            isConnecting = false;
            reconnectAttempts = 0;
            refreshRoomList();
            
            // 启动心跳检测
            startHeartbeat();
        };
        
        ws.onmessage = function(event) {
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
                // 重置心跳计时器
                resetHeartbeat();
                handleWebSocketMessage(message);
            } catch (error) {
                console.error('解析WebSocket消息失败:', error);
            }
        };
        
        ws.onclose = function(event) {
            console.log('WebSocket连接已关闭, 代码:', event.code, '原因:', event.reason);
            isConnecting = false;
            stopHeartbeat();
            
            // 清理连接资源
            if (ws) {
                ws.close();
                ws = null;
            }

            handleReconnect(event);
        };
        
        ws.onerror = function(error) {
            console.error('WebSocket错误:', error);
            isConnecting = false;
        };
    } catch (error) {
        console.error('初始化WebSocket失败:', error);
        isConnecting = false;
        handleReconnect();
    }
}

// 处理重连逻辑
function handleReconnect(event) {
    // 只在非正常关闭时尝试重连
    if (!event || (event.code !== 1000 && event.code !== 1001)) {
        if (document.visibilityState === 'visible' && reconnectAttempts < maxReconnectAttempts) {
            reconnectAttempts++;
            const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000);
            console.log(`尝试重新连接 (${reconnectAttempts}/${maxReconnectAttempts}), 等待${delay/1000}秒...`);
            setTimeout(initWebSocket, delay);
        } else if (reconnectAttempts >= maxReconnectAttempts) {
            console.error('WebSocket重连次数超过最大限制');
            $.messager.alert('错误', '网络连接失败，请刷新页面重试');
        }
    }
}

// 心跳检测
let heartbeatInterval = null;
let heartbeatTimeout = null;

function startHeartbeat() {
    stopHeartbeat(); // 确保之前的心跳已停止
    
    // 每15秒发送一次心跳
    heartbeatInterval = setInterval(() => {
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'ping' }));
            
            // 如果5秒内没有收到响应，认为连接已断开
            heartbeatTimeout = setTimeout(() => {
                console.error('心跳检测超时');
                if (ws) ws.close();
            }, 5000);
        }
    }, 15000);
}

function resetHeartbeat() {
    if (heartbeatTimeout) {
        clearTimeout(heartbeatTimeout);
        heartbeatTimeout = null;
    }
}

function stopHeartbeat() {
    if (heartbeatInterval) {
        clearInterval(heartbeatInterval);
        heartbeatInterval = null;
    }
    resetHeartbeat();
}

// 处理WebSocket消息
function handleWebSocketMessage(message) {
    switch(message.type) {
        case 'room_update':
            refreshRoomList();
            break;
        case 'game_start':
            window.location.href = `/game?room=${message.content.roomId}`;
            break;
        default:
            console.log('未知消息类型:', message.type);
    }
}

// 刷新房间列表
function refreshRoomList() {
    fetch('/api/rooms')
        .then(response => response.json())
        .then(data => {
            $('#roomList').datagrid('loadData', data.rooms);
        })
        .catch(error => {
            $.messager.alert('错误', '获取房间列表失败: ' + error.message);
        });
}

// 创建房间
function createRoom() {
    if (!validatePlayerName()) return;

    $('#createRoomDialog').dialog({
        title: '创建房间',
        width: 400,
        height: 250,
        closed: false,
        cache: false,
        modal: true,
        buttons: [{
            text: '创建',
            handler: function() {
                const roomData = {
                    name: $('#roomName').val(),
                    mode: $('#gameMode').val(),
                    max_players: parseInt($('#maxPlayers').val())
                };
                
                fetch('/api/rooms', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(roomData)
                })
                .then(response => response.json())
                .then(data => {
                    $('#createRoomDialog').dialog('close');
                    refreshRoomList();
                    
                    // 生成并保存玩家信息
                    const playerData = {
                        name: $('#playerName').val(),
                        id: generatePlayerId()
                    };
                    localStorage.setItem('playerId', playerData.id);
                    localStorage.setItem('playerName', playerData.name);
                    
                    // 加入房间
                    return fetch(`/api/rooms/${data.id}/join`, {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json'
                        },
                        body: JSON.stringify(playerData)
                    })
                    .then(response => {
                        if (!response.ok) {
                            throw new Error('加入房间失败');
                        }
                        return response.json().then(() => data);
                    });
                })
                .then(data => {
                    // 跳转到游戏页面
                    window.location.href = `/game?room=${data.id}&player=${localStorage.getItem('playerId')}`;
                })
                .catch(error => {
                    $.messager.alert('错误', '创建房间失败: ' + error.message);
                });
            }
        }, {
            text: '取消',
            handler: function() {
                $('#createRoomDialog').dialog('close');
            }
        }]
    });
}

// 加入房间
function joinRoom(roomId) {
    if (!validatePlayerName()) return;

    const row = roomId ? { id: roomId } : $('#roomList').datagrid('getSelected');
    if (!row) {
        $.messager.alert('提示', '请先选择一个房间');
        return;
    }
    
    const playerData = {
        name: $('#playerName').val(),
        id: generatePlayerId()
    };
    
    fetch(`/api/rooms/${row.id}/join`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(playerData)
    })
    .then(response => response.json())
    .then(data => {
        // 保存玩家信息
        localStorage.setItem('playerId', playerData.id);
        localStorage.setItem('playerName', playerData.name);
        
        // 跳转到游戏房间
        window.location.href = `/game?room=${row.id}&player=${playerData.id}`;
    })
    .catch(error => {
        $.messager.alert('错误', '加入房间失败: ' + error.message);
    });
}

// 验证玩家名称
function validatePlayerName() {
    const playerName = $('#playerName').val().trim();
    if (!playerName) {
        $.messager.alert('提示', '请输入玩家昵称');
        return false;
    }
    return true;
}

// 生成玩家ID
function generatePlayerId() {
    return 'player_' + Math.random().toString(36).substr(2, 9);
}

// 页面加载完成后初始化
$(document).ready(function() {
    // 只在用户成功加入房间后初始化WebSocket
    if (window.location.pathname === '/game') {
        const urlParams = new URLSearchParams(window.location.search);
        const roomId = urlParams.get('room');
        const playerId = urlParams.get('player');
        
        if (roomId && playerId) {
            initWebSocket();
        }
    }
    
    refreshRoomList();
    
    // 每30秒刷新一次房间列表
    setInterval(refreshRoomList, 30000);
});