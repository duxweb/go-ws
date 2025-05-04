package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	websocket "github.com/duxweb/go-ws"
	"github.com/duxweb/go-ws/drivers"
)

func main() {

	// 创建一个Provider
	provider := &websocket.Provider{
		Auth: func(token string) (map[string]any, error) {
			// 简单的认证逻辑，直接使用token作为用户名
			if token == "" {
				return nil, fmt.Errorf("请输入用户名")
			}

			// 打印用户连接信息
			fmt.Printf("用户 [%s] 正在连接\n", token)

			// 返回客户端ID和其他数据
			return map[string]any{
				"client_id": token, // 使用token作为客户端ID
				"user_name": token, // 使用token作为用户名
			}, nil
		},
		Event: func(name string, client *websocket.Client) error {
			// 处理事件：上线、下线、ping
			fmt.Printf("客户端 %s 触发事件: %s\n", client.ClientID, name)

			// 如果是上线事件，发送欢迎消息
			if name == "online" {
				// 使用客户端ID作为用户名（因为前面已设置token为用户名）
				userName := client.ClientID

				client.Service.SendLocalClient(client.ClientID, &websocket.Message{
					Type:    "system",
					Message: fmt.Sprintf("欢迎 %s，连接成功!", userName),
					ID:      fmt.Sprintf("%d", time.Now().UnixMilli()), // 添加时间戳ID
				})

				// 打印用户上线信息
				fmt.Printf("用户 [%s] 已成功连接\n", userName)
			}

			// 如果是下线事件，打印日志
			if name == "offline" {
				fmt.Printf("用户 [%s] 已断开连接\n", client.ClientID)
			}

			return nil
		},
		Message: func(message *websocket.Message, client *websocket.Client) error {
			// 处理接收到的消息
			fmt.Printf("收到客户端 %s 的消息: %s\n", client.ClientID, message.Message)

			if message.Type == "message" {

				// 创建消息载荷
				payload := &websocket.Message{
					Type:    "message",
					Message: message.Message,
					ID:      message.ID,
					Meta:    message.Meta,
				}

				// 将消息广播到频道
				client.Push([]string{message.Channel}, payload)
			}

			if message.Type == "subscribe" {
				client.Sub(message.Channel)
			}

			if message.Type == "unsubscribe" {
				client.Unsub(message.Channel)
			}

			return nil
		},
	}

	//memoryDriver := drivers.NewMemoryDriver()

	redisDriver, err := drivers.NewRedisDriver(&drivers.RedisOptions{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	if err != nil {
		log.Fatal(err)
	}

	// 初始化WebSocket服务
	service := websocket.New(redisDriver, provider)

	service.Run()

	// 设置HTTP服务器
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>WebSocket测试</title>
			<style>
				body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
				.panel { margin-bottom: 20px; border: 1px solid #eee; padding: 15px; border-radius: 5px; }
				#log { height: 300px; overflow-y: scroll; border: 1px solid #ccc; margin-top: 10px; padding: 10px; background: #f8f8f8; }
				input, button { padding: 8px; margin: 5px 0; }
				button { cursor: pointer; background: #4CAF50; color: white; border: none; border-radius: 4px; }
				button:hover { background: #45a049; }
				.message { padding: 5px 0; border-bottom: 1px solid #eee; }
				.system { color: blue; }
				.error { color: red; }
				.success { color: green; }
				.broadcast { color: purple; }
				.connection-status { padding: 5px 10px; border-radius: 3px; display: inline-block; margin-left: 10px; }
				.connected { background: #dff0d8; color: #3c763d; }
				.disconnected { background: #f2dede; color: #a94442; }
				.form-group { margin-bottom: 10px; }
				.form-group label { display: inline-block; width: 80px; }
			</style>
		</head>
		<body>
			<h1>WebSocket测试页面</h1>
			<div class="panel">
				<h3>连接状态 <span id="connection-status" class="connection-status disconnected">未连接</span></h3>
				<div class="form-group">
					<label for="username">用户名:</label>
					<input id="username" placeholder="请输入用户名" value="测试用户" />
				</div>
				<button onclick="connect()">连接</button>
				<button onclick="disconnect()">断开</button>
			</div>
			<div class="panel">
				<h3>频道</h3>
				<input id="channel" placeholder="频道名称" value="channel1" />
				<button onclick="subscribe()">订阅</button>
				<button onclick="unsubscribe()">取消订阅</button>
			</div>
			<div class="panel">
				<h3>消息</h3>
				<input id="message" placeholder="请输入要发送的消息" />
				<button onclick="sendMessage()">发送</button>
			</div>
			<div class="panel">
				<h3>日志</h3>
				<div id="log"></div>
			</div>

			<script>
				let ws;

				function log(message, className = '') {
					const logDiv = document.getElementById('log');
					const msgDiv = document.createElement('div');
					msgDiv.className = 'message ' + className;
					msgDiv.innerHTML = message;
					logDiv.appendChild(msgDiv);
					logDiv.scrollTop = logDiv.scrollHeight;
				}

				function updateConnectionStatus(connected) {
					const statusElem = document.getElementById('connection-status');
					if (connected) {
						statusElem.className = 'connection-status connected';
						statusElem.textContent = '已连接: ' + currentUserData.username;
					} else {
						statusElem.className = 'connection-status disconnected';
						statusElem.textContent = '未连接';
					}
				}

				let currentUserData = {};

				function connect() {
					const username = document.getElementById('username').value;

					if (!username) {
						log('请输入用户名', 'error');
						return;
					}

					// 保存当前用户数据
					currentUserData = {
						username: username
					};

					// 直接使用用户名作为token
					const token = encodeURIComponent(username);

					ws = new WebSocket('ws://localhost:8080/ws?app=app1&token=' + token);
					ws.onopen = function() {
						log('连接已建立，等待服务器验证...', 'system');
					};

					ws.onmessage = function(e) {
						const data = JSON.parse(e.data);
						let className = data.type || '';
						let messageText = '';

						// 记录接收到的所有消息
						console.log('收到消息:', data);

						// 连接成功时更新状态
						if (data.type === 'system' && data.message && data.message.includes('欢迎') && data.message.includes('连接成功')) {
							updateConnectionStatus(true);
							messageText = data.message;
						} else if (data.type === 'message' && data.message) {
							// 显示发送者的用户名和消息内容
							const username = data.meta && data.meta.username ? data.meta.username : '未知用户';
							messageText = username + ': ' + data.message;
						} else if (data.message) {
							// 如果有消息内容但不是以上类型
							messageText = data.message;
						} else {
							// 其他情况，显示完整JSON
							messageText = JSON.stringify(data);
						}

						log(messageText, className);
					};

					ws.onclose = function() {
						log('连接已关闭', 'system');
						updateConnectionStatus(false);
					};

					ws.onerror = function(e) {
						log('连接错误: ' + e.message, 'error');
						updateConnectionStatus(false);
					};
				}

				function disconnect() {
					if (ws) {
						ws.close();
						log('已断开连接', 'system');
						updateConnectionStatus(false);
					}
				}

				function subscribe() {
					if (!ws) {
						log('请先建立连接', 'error');
						return;
					}
					const channel = document.getElementById('channel').value;
					if (!channel) {
						log('请输入频道名称', 'error');
						return;
					}
					ws.send(JSON.stringify({
						type: 'subscribe',
						channel: channel,
						id: Date.now().toString(),
					}));
					log('已发送订阅请求: ' + channel);
				}

				function unsubscribe() {
					if (!ws) {
						log('请先建立连接', 'error');
						return;
					}
					const channel = document.getElementById('channel').value;
					if (!channel) {
						log('请输入频道名称', 'error');
						return;
					}
					ws.send(JSON.stringify({
						type: 'unsubscribe',
						channel: channel,
						id: Date.now().toString(),
					}));
					log('已发送取消订阅请求: ' + channel);
				}

				function sendMessage() {
					if (!ws) {
						log('请先建立连接', 'error');
						return;
					}
					const channel = document.getElementById('channel').value;
					const message = document.getElementById('message').value;
					if (!channel) {
						log('请输入频道名称', 'error');
						return;
					}
					if (!message) {
						log('请输入要发送的消息', 'error');
						return;
					}
					ws.send(JSON.stringify({
						id: Date.now().toString(),
						type: 'message',
						channel: channel,
						message: message,
						// 添加用户名
						meta: {
							username: currentUserData.username
						}
					}));
					log('已发送消息到频道 ' + channel + ': ' + message);
					// 清空消息输入框
					document.getElementById('message').value = '';
				}
			</script>
		</body>
		</html>
		`))
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// 从URL参数中获取token（用户名）
		token := r.URL.Query().Get("token")
		if token != "" {
			// 将token直接添加到请求头部
			r.Header.Set("Token", token)
			fmt.Printf("用户 [%s] 尝试连接\n", token)
		}

		// 确保service.Websocket已初始化
		if service == nil || service.Websocket == nil {
			http.Error(w, "WebSocket服务未初始化", http.StatusInternalServerError)
			return
		}

		// 处理WebSocket请求
		service.Websocket.HandleRequest(w, r)
	})

	// 启动HTTP服务器
	fmt.Println("WebSocket服务器运行在 http://localhost:8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
