package websocket

import (
	"container/list"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"github.com/olahol/melody"
	"github.com/spf13/cast"
)

// Client 客户端映射
// Client mapping
type Client struct {
	App string // 应用
	// Application
	ClientID string // 客户端ID
	// Client ID
	Token string // 客户授权 token
	// Client authorization token
	Conn *melody.Session // ws 连接
	// WebSocket connection
	Channel *listLock // 频道列表
	// Channel list
	Data map[string]any // 附加数据 - 登录授权传递
	// Additional data - passed during login authorization
	Service *ServiceT // 服务
	// Service
}

// Channel 频道映射
// Channel mapping
type Channel struct {
	ClientID string
	Channel  *listLock
}

// listLock 线程安全的列表
// listLock thread-safe list
type listLock struct {
	list.List
	sync.Mutex
}

// GetClient 获取客户端
// GetClient retrieves a client by ID
func (t *ServiceT) GetClient(clientID string) (*Client, error) {
	lastClient, ok := t.Clients.Load(clientID)
	if !ok {
		return nil, errors.New("client is not online")
	}
	return lastClient.(*Client), nil
}

// SendClient 给客户端发消息
// SendClient sends a message to a client
func (t *ServiceT) SendClient(clientID string, message *Message) error {
	client, err := t.GetClient(clientID)
	if err != nil {
		slog.Error("Websocket Send Client", slog.Any("error", err), slog.Any("message", message))
		return err
	}
	slog.Debug("Websocket Send Client", slog.String("client", clientID), slog.Any("message", message))
	content, _ := json.Marshal(message)
	err = client.Conn.Write(content)
	if err != nil {
		return err
	}
	return nil
}

// AddClient 添加客户端
// AddClient adds a new client to the service
func (t *ServiceT) AddClient(app string, client *melody.Session, clientID string, token string, data map[string]any) {
	client.Set("clientID", clientID)
	slog.Info("Websocket Add Client", slog.String("client", clientID))
	t.Clients.Store(clientID, &Client{
		Conn:     client,
		ClientID: clientID,
		Channel:  &listLock{},
		App:      app,
		Token:    token,
		Data:     data,
		Service:  t,
	})
}

// RemoveConnClient 移除客户端
// RemoveConnClient removes a client by connection
func (t *ServiceT) RemoveConnClient(conn *melody.Session) {
	clientID, ok := conn.Get("clientID")
	if !ok {
		return
	}
	t.RemoveClient(cast.ToString(clientID))
}

// RemoveClient 移除客户端
// RemoveClient removes a client by ID
func (t *ServiceT) RemoveClient(clientID string) {
	client, err := t.GetClient(clientID)
	if err != nil {
		return
	}
	_ = client.Conn.CloseWithMsg([]byte("Client disconnected"))

	// 取消全部订阅
	// Cancel all subscriptions
	client.Unsub()

	t.Clients.Delete(clientID)
	slog.Info("Websocket Del Client", slog.String("client", clientID))
}
