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
func New(driver Driver, provider *Provider) *Service {
	return &Service{
		Websocket: melody.New(),
		Clients:   &sync.Map{},
		Channels:  &sync.Map{},
		Provider:  provider,
		Driver:    driver,
	}
}

// ServiceT WebSocket服务主结构
// ServiceT main WebSocket service structure
type Service struct {
	Websocket *melody.Melody // WebSocket引擎
	// WebSocket engine
	Clients *sync.Map // 客户端映射表
	// Client mapping table
	Channels *sync.Map // 频道映射表
	// Channel mapping table
	// 应用提供者映射表
	// Application provider mapping table
	Provider *Provider
	// 订阅驱动
	// Subscribe driver
	Driver Driver
}

// Message WebSocket消息结构
// Message WebSocket message structure
type Message struct {
	ID string `json:"id"` // 消息ID
	// Message ID
	Type string `json:"type"` // 消息类型
	// Message type
	ClientID string `json:"client,omitempty"` // 客户端ID
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

// Run 启动WebSocket服务
// Run starts the WebSocket service
func (t *Service) Run() {
	// 连接处理
	// Connection handling
	t.Websocket.HandleConnect(func(session *melody.Session) {
		token := session.Request.Header.Get("token")
		if t.Provider == nil {
			msg := "applications are not registered"
			slog.Error(msg)
			_ = session.CloseWithMsg([]byte(msg))
		}
		data, err := t.Provider.Auth(token)
		if err != nil {
			slog.Error("Websocket Auth", slog.Any("error", err))
			_ = session.CloseWithMsg([]byte(err.Error()))
			return
		}
		clientID := data["client_id"].(string)

		// 把之前的客户端踢下线
		// Kick out previous client with the same ID
		t.RemoveLocalClient(clientID)

		// 创建新客户端
		// Create new client
		t.AddLocalClient(session, clientID, token, data)

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
		client, err := t.GetLobalClient(clientID)
		if err != nil {
			return
		}

		// 处理消息
		// Process message
		err = t.Provider.Message(&data, client)
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
			Meta: map[string]any{
				"time": time.Now().Format("2006-01-02 15:04:05"),
			},
		}

		slog.Debug("Websocket Send", slog.Any("data", msgData))
		t.SendLocalClient(clientID, &msgData)
	})
}
