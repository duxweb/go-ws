# go-ws

go-ws 是一个基于 Golang 的 WebSocket 通讯库，为开发实时通信应用提供简单易用的接口。该库将 Websocket 封装成认证、事件、频道、订阅方法，无需关心内部通讯即可利用频道订阅方式接收发送消息。

go-ws is a Golang-based WebSocket communication library that provides simple and user-friendly interfaces for developing real-time communication applications. This library encapsulates Websocket into authentication, events, channels, and subscription methods, allowing you to receive and send messages via channel subscriptions without worrying about internal communication.


## 分布式

目前 go-ws 尚不支持分布式部署，仅适用于单机环境。在单机环境中，WebSocket 连接、频道管理和消息推送都在同一进程内进行处理。

## 特性
## Features

- WebSocket 连接管理 (WebSocket connection management)
- 客户端认证机制 (Client authentication mechanism)
- 频道订阅/取消订阅 (Channel subscription/unsubscription)
- 实时消息推送 (Real-time message pushing)
- 客户端在线状态管理 (Client online status management)
- 可扩展的应用提供者系统 (Extensible application provider system)

## 架构流程图

```mermaid
graph TD
    %% 核心组件
    WebSocket[WebSocket服务] --> Provider[Provider]
    Provider --> |认证/事件/消息| Client[客户端实例]
    WebSocket --> ChannelMgr[频道管理器]

    %% 订阅流程
    Client1[客户端1] --> |连接| WebSocket
    Client1 --> |订阅频道A| Sub[订阅请求]
    Sub --> Client.Sub[client.Sub方法]
    Client.Sub --> |注册| ChannelMgr

    %% 推送流程
    Client1 --> |发送消息到频道A| PushMsg[消息]
    PushMsg --> Provider
    Provider --> |处理消息| Push[websocket.Push]
    Push --> |查询订阅者| ChannelMgr
    ChannelMgr --> |找到订阅频道A的客户端| Deliver[推送消息]
    Deliver --> Client2[客户端2]
    Deliver --> Client3[客户端3]

    %% 简化样式
    classDef core fill:#f9f,stroke:#333,stroke-width:2px;
    classDef client fill:#bbf,stroke:#333,stroke-width:1px;
    classDef important fill:#fbb,stroke:#333,stroke-width:2px,stroke-dasharray: 5 5;

    class WebSocket,Provider,ChannelMgr core;
    class Client1,Client2,Client3 client;
    class Sub,Push important;
```

## 安装

```bash
go get github.com/duxweb/go-ws
```

## 快速开始

### 初始化 WebSocket 服务

```go
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
	// 初始化WebSocket服务
	service := websocket.New(drivers.NewMemorySubscribeDriver())
	service.Run()

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
				// 使用客户端ID作为用户名
				userName := client.ClientID

				client.Send(&websocket.Message{
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

	// 注册Provider
	service.RegisterProvider("app1", provider)

	// 设置HTTP服务器
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 内置测试界面HTML代码...
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// 从URL参数中获取token（用户名）
		token := r.URL.Query().Get("token")
		if token != "" {
			// 将token直接添加到请求头部
			r.Header.Set("Token", token)
		}

		// 处理WebSocket请求
		service.Websocket.HandleRequest(w, r)
	})

	// 启动HTTP服务器
	fmt.Println("WebSocket服务器运行在 http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### 前端连接示例

```javascript
// 创建WebSocket连接
const username = "测试用户";
const ws = new WebSocket(`ws://localhost:8080/ws?app=app1&token=${encodeURIComponent(username)}`);

// 连接建立后
ws.onopen = function() {
    console.log('连接已建立，等待服务器验证...');
};

// 接收消息
ws.onmessage = function(e) {
    const data = JSON.parse(e.data);
    console.log('收到消息:', data);
};

// 订阅频道
function subscribe(channel) {
    ws.send(JSON.stringify({
        type: 'subscribe',
        channel: channel,
        id: Date.now().toString(),
    }));
}

// 取消订阅频道
function unsubscribe(channel) {
    ws.send(JSON.stringify({
        type: 'unsubscribe',
        channel: channel,
        id: Date.now().toString(),
    }));
}

// 发送消息
function sendMessage(channel, message) {
    ws.send(JSON.stringify({
        id: Date.now().toString(),
        type: 'message',
        channel: channel,
        message: message,
        meta: {
            username: username
        }
    }));
}
```

### 消息格式

#### 客户端发送的消息格式

```json
// 发送聊天消息
{
  "id": "时间戳",
  "type": "message",
  "channel": "频道名称",
  "message": "消息内容",
  "meta": {
    "username": "用户名"
  }
}

// 订阅频道
{
  "type": "subscribe",
  "channel": "频道名称",
  "id": "时间戳"
}

// 取消订阅
{
  "type": "unsubscribe",
  "channel": "频道名称",
  "id": "时间戳"
}
```

#### 服务器发送的消息格式

```json
// 系统消息
{
  "type": "system",
  "message": "系统消息内容",
  "id": "时间戳"
}

// 用户消息
{
  "type": "message",
  "message": "消息内容",
  "id": "消息ID",
  "meta": {
    "username": "发送者用户名"
  }
}
```

## 核心概念
## Core Concepts

### Provider

Provider 是应用逻辑的提供者，包含三个主要处理函数：

Provider is the application logic provider, containing three main handler functions:

- **Auth**: 处理客户端认证
  - 函数签名: `func(token string) (map[string]any, error)`
  - 参数: `token` - 客户端提供的认证令牌
  - 返回: 包含客户端信息的map（必须包含`client_id`键）和可能的错误
  - 用途: 验证客户端身份，并提供客户端标识和元数据

- **Auth**: Handle client authentication
  - Signature: `func(token string) (map[string]any, error)`
  - Parameters: `token` - Authentication token provided by client
  - Returns: Map containing client information (must include `client_id` key) and possible error
  - Purpose: Validate client identity and provide client identification and metadata

- **Event**: 处理客户端事件（online、offline、ping）
  - 函数签名: `func(name string, client *Client) error`
  - 参数: `name` - 事件名称, `client` - 客户端实例
  - 支持的事件:
    - `online`: 客户端连接成功并认证通过
    - `offline`: 客户端断开连接
    - `ping`: 客户端发送心跳包
  - 用途: 响应客户端状态变化，执行相应的业务逻辑

- **Event**: Handle client events (online, offline, ping)
  - Signature: `func(name string, client *Client) error`
  - Parameters: `name` - Event name, `client` - Client instance
  - Supported events:
    - `online`: Client connected successfully and authenticated
    - `offline`: Client disconnected
    - `ping`: Client sent heartbeat
  - Purpose: Respond to client status changes and execute corresponding business logic

- **Message**: 处理客户端发送的消息
  - 函数签名: `func(message *Message, client *Client) error`
  - 参数: `message` - 消息数据, `client` - 客户端实例
  - 用途: 解析和处理客户端发送的消息，执行相应的业务逻辑，如消息转发、数据处理等
  - 常见操作: 根据消息类型(`message.Type`)执行不同操作，如订阅频道、发送消息等

- **Message**: Handle messages sent by client
  - Signature: `func(message *Message, client *Client) error`
  - Parameters: `message` - Message data, `client` - Client instance
  - Purpose: Parse and process messages sent by client, execute corresponding business logic like message forwarding, data processing, etc.
  - Common operations: Perform different actions based on message type (`message.Type`), such as subscribing to channels, sending messages, etc.

### Client

Client 代表一个已连接的客户端，提供以下属性和方法：

Client represents a connected client and provides the following attributes and methods:

- **ClientID**: 客户端唯一标识
  Client unique identifier
- **App**: 客户端所属应用
  Application the client belongs to
- **Token**: 客户端认证Token
  Client authentication token
- **Data**: 客户端附加数据
  Client additional data
- **Sub(channels...)**: 订阅频道
  Subscribe to channels
- **Unsub(channels...)**: 取消订阅频道
  Unsubscribe from channels
- **Send(data)**: 发送消息给客户端
  Send message to client
- **Push(channels, data)**: 向频道推送消息
  Push message to channels

### 频道订阅与推送
### Channel Subscription and Push

```go
// 订阅频道
// Subscribe to channels
client.Sub("channel1", "channel2")

// 向频道推送消息
// Push message to channels
websocket.Push([]string{"channel1"}, map[string]any{
    "type": "notification",
    "message": "频道消息",
})
```

### 简化流程
### Simplified Process

1. **注册Provider**: 配置认证、事件和消息处理逻辑
   **Register Provider**: Configure authentication, event, and message handling logic

2. **客户端连接**: 通过Provider认证后创建Client实例
   **Client Connection**: Create Client instance after authentication through Provider

3. **订阅频道**: 客户端调用Sub方法订阅感兴趣的频道
   **Subscribe to Channels**: Client calls Sub method to subscribe to channels of interest

4. **消息处理**: Provider处理客户端发送的消息
   **Message Processing**: Provider processes messages sent by client

5. **消息推送**: 通过Push方法向频道推送消息，自动分发给所有订阅者
   **Message Push**: Push messages to channels through Push method, automatically distributing to all subscribers

## 完整示例

完整的示例可以在 `example` 目录中找到。

## 许可证

MIT 许可证