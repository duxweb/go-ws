package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/olahol/melody"
	"github.com/spf13/cast"
)

// New 创建新的WebSocket服务实例
// New creates a new WebSocket service instance
func New(driver SubscribeDriver) *ServiceT {
	return &ServiceT{
		Websocket: melody.New(),
		Clients:   &sync.Map{},
		Channels:  &sync.Map{},
		Providers: map[string]*Provider{},
		Driver:    driver,
	}
}

// Provider 应用程序提供者，处理认证、事件和消息
// Provider is an application provider that handles authentication, events, and messages
type Provider struct {
	// Auth 客户端认证处理方法
	// Auth client authentication handler
	// 参数: token - 客户端提供的认证令牌
	// Params: token - authentication token provided by client
	// 返回: 包含客户端信息的map和可能的错误
	// Returns: map containing client information and possible error
	Auth func(token string) (map[string]any, error)

	// Event 客户端事件处理方法
	// Event client event handler
	// 支持的事件: online(上线), offline(离线), ping(心跳)
	// Supported events: online, offline, ping
	// 参数: name - 事件名称, client - 客户端实例
	// Params: name - event name, client - client instance
	// 返回: 可能的错误
	// Returns: possible error
	Event func(name string, client *Client) error

	// Message 客户端消息处理方法
	// Message client message handler
	// 参数: message - 消息数据, client - 客户端实例
	// Params: message - message data, client - client instance
	// 返回: 可能的错误
	// Returns: possible error
	Message func(message *Message, client *Client) error
}

// ServiceT WebSocket服务主结构
// ServiceT main WebSocket service structure
type ServiceT struct {
	Websocket *melody.Melody // WebSocket引擎
	// WebSocket engine
	Clients *sync.Map // 客户端映射表
	// Client mapping table
	Channels *sync.Map // 频道映射表
	// Channel mapping table
	Providers map[string]*Provider // 应用提供者映射表
	// Application provider mapping table
	Driver SubscribeDriver // 订阅驱动
}

// Message WebSocket消息结构
// Message WebSocket message structure
type Message struct {
	ID string `json:"id"` // 消息ID
	// Message ID
	Type string `json:"type"` // 消息类型
	// Message type
	Client string `json:"client,omitempty"` // 客户端ID
	// Client ID
	Channel string `json:"channel"` // 频道
	// Channel
	Message string `json:"message"` // 消息内容
	// Message content
	Data any `json:"data,omitempty"` // 附加数据
	// Additional data
	Meta any `json:"meta,omitempty"` // 元数据
	// Metadata
}

// RegisterProvider 注册应用提供者
// RegisterProvider registers an application provider
func (t *ServiceT) RegisterProvider(name string, provider *Provider) {
	t.Providers[name] = provider
}

// Run 启动WebSocket服务
// Run starts the WebSocket service
func (t *ServiceT) Run() {
	// 连接处理
	// Connection handling
	t.Websocket.HandleConnect(func(session *melody.Session) {
		token := session.Request.Header.Get("token")
		app := session.Request.URL.Query().Get("app")
		provider, ok := t.Providers[app]
		if !ok {
			msg := "applications are not registered"
			slog.Error(msg)
			_ = session.CloseWithMsg([]byte(msg))
		}
		data, err := provider.Auth(token)
		if err != nil {
			slog.Error("Websocket Auth", slog.Any("error", err))
			_ = session.CloseWithMsg([]byte(err.Error()))
			return
		}
		clientID := data["client_id"].(string)

		// 把之前的客户端踢下线
		// Kick out previous client with the same ID
		t.RemoveClient(clientID)

		// 创建新客户端
		// Create new client
		t.AddClient(app, session, clientID, token, data)

		// 设置客户端在线
		// Set client online
		err = t.EventOnline(clientID)
		if err != nil {
			slog.Error("Websocket Client Online", slog.Any("error", err))
			_ = session.CloseWithMsg([]byte(err.Error()))
			return
		}
	})

	// 销毁处理
	// Disconnect handling
	t.Websocket.HandleDisconnect(func(session *melody.Session) {
		str, ok := session.Get("clientID")
		if !ok {
			return
		}
		clientID := cast.ToString(str)
		slog.Info("Websocket Client Disconnect", slog.String("client", clientID))

		// 移除客户端
		// Remove client
		t.RemoveConnClient(session)

		// 发送离线
		// Send offline event
		err := t.EventOffline(clientID)
		if err != nil {
			slog.Error("Websocket Client Offline", slog.Any("error", err))
		}
	})

	// ping 处理
	// Ping handling
	t.Websocket.HandlePong(func(session *melody.Session) {
		str, ok := session.Get("clientID")
		if !ok {
			return
		}
		clientID := cast.ToString(str)
		slog.Debug("Websocket Ping", slog.String("client", clientID))
		_ = t.EventPing(clientID)
	})

	// 收到消息
	// Message received handling
	t.Websocket.HandleMessage(func(s *melody.Session, msg []byte) {
		name, _ := s.Get("clientID")
		clientID := cast.ToString(name)
		slog.Debug("Websocket Received", slog.String("client", clientID), slog.String("message", string(msg)))

		data := Message{}
		err := json.Unmarshal(msg, &data)
		if err != nil {
			slog.Error("Websocket Received", slog.Any("error", err), slog.String("client", clientID), slog.String("message", string(msg)))
			return
		}
		client, err := t.GetClient(clientID)
		if err != nil {
			return
		}

		provider := t.Providers[client.App]
		err = provider.Message(&data, client)
		if err != nil {
			slog.Error("Websocket Received", slog.Any("error", err), slog.String("client", clientID), slog.String("message", string(msg)))
			return
		}

		slog.Debug("Websocket Received", slog.Any("data", data))

		// 消息反馈
		// Message feedback
		msgData := Message{
			ID:   data.ID,
			Type: "receive",
			Data: map[string]any{
				"id":   data.ID,
				"time": time.Now().Format("2006-01-02 15:04:05"),
			},
		}

		slog.Debug("Websocket Send", slog.Any("data", msgData))
		client.Send(&msgData)
	})
}
