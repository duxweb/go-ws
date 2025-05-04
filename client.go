package websocket

import (
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
	// 客户端ID
	// Client ID
	ClientID string
	// 客户授权 token
	// Client authorization token
	Token string
	// Conn Ws 连接
	// Conn connection
	Conn *melody.Session
	// 附加数据 - 登录授权传递
	// Additional data - passed during login authorization
	Data map[string]any
	// 服务
	// Service
	Service *Service

	// 锁
	// Lock
	lock sync.RWMutex
}

// Sub 订阅主题
// Sub subscribes to topics
func (p *Client) Sub(topics ...string) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if len(topics) < 1 {
		return errors.New("empty topic")
	}
	slog.Info("Websocket Client Sub", slog.String("client", p.ClientID), slog.Any("topics", topics))

	for _, topicID := range topics {

		// 订阅主题
		// Create topic
		err := p.Service.SubTopic(topicID)
		if err != nil {
			slog.Error("Failed to create topic", slog.Any("error", err))
		}

		// 客户端订阅主题
		// Subscribe to topic
		err = p.Service.ClientToTopic(p.ClientID, topicID)
		if err != nil {
			slog.Error("Failed to add client to topic", slog.Any("error", err))
		}
	}
	return nil
}

// Unsub 取消订阅主题
// Unsub unsubscribes from topics
func (p *Client) Unsub(args ...string) {
	var topics = make([]string, 0)
	var err error

	if len(args) <= 0 {
		topics, err = p.Service.GetClientTopics(p.ClientID)
		if err != nil {
			slog.Error("Failed to get client topics", slog.Any("error", err))
		}
	} else {
		topics = args
	}

	slog.Info("Websocket Client Unsub", slog.String("client", p.ClientID), slog.Any("topics", topics))

	for _, topic := range topics {
		err = p.Service.UnsubClientFromTopic(topic, p.ClientID)
		if err != nil {
			slog.Error("Failed to remove client from topic", slog.Any("error", err))
		}

		// 判断是否还有客户端订阅主题
		// Check if there are any clients subscribed to the topic
		clients, err := p.Service.GetTopicClients(topic)
		if err != nil {
			slog.Error("Failed to get topic clients", slog.Any("error", err))
		}
		if len(clients) == 0 {
			// 没有客户端订阅主题，删除主题
			// No clients subscribed to the topic, delete the topic
			err = p.Service.UnsubTopic(topic)
			if err != nil {
				slog.Error("Failed to delete topic", slog.Any("error", err))
			}
		}
	}
}

// Push 发布消息到主题
// Push publishes a message to topics
func (p *Client) Push(topics []string, data *Message) {
	for _, topic := range topics {
		msg := &Message{
			ID:       data.ID,
			Type:     data.Type,
			ClientID: data.ClientID,
			Channel:  topic,
			Message:  data.Message,
			Data:     data.Data,
			Meta:     data.Meta,
		}

		// 发布消息到主题
		// Publish a message to a topic
		p.Service.Driver.Publish(msg)
	}
}

// GetClient 获取客户端
// GetClient retrieves a client by ID
func (t *Service) GetLobalClient(clientID string) (*Client, error) {
	lastClient, ok := t.Clients.Load(clientID)
	if !ok {
		return nil, errors.New("client is not online")
	}
	return lastClient.(*Client), nil
}

func (t *Service) IsLocalClient(clientID string) bool {
	_, ok := t.Clients.Load(clientID)
	return ok
}

// SendClient 给客户端发消息
// SendClient sends a message to a client
func (t *Service) SendLocalClient(clientID string, message *Message) error {
	client, err := t.GetLobalClient(clientID)
	if err != nil {
		slog.Info("Websocket Send Client", slog.Any("error", err), slog.Any("message", message))
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
func (t *Service) AddLocalClient(client *melody.Session, clientID string, token string, data map[string]any) {
	client.Set("clientID", clientID)
	slog.Info("Websocket Add Client", slog.String("client", clientID))

	// 创建客户端数据
	// Create client data
	clientData := &Client{
		Conn:     client,
		ClientID: clientID,
		Token:    token,
		Data:     data,
		Service:  t,
	}

	// 存储到内存
	// Store in memory
	t.Clients.Store(clientID, clientData)
}

// RemoveConnClient 移除客户端
// RemoveConnClient removes a client by connection
func (t *Service) RemoveConnClient(conn *melody.Session) {
	clientID, ok := conn.Get("clientID")
	if !ok {
		return
	}
	t.RemoveLocalClient(cast.ToString(clientID))
}

// RemoveClient 移除客户端
// RemoveClient removes a client by ID
func (t *Service) RemoveLocalClient(clientID string) {
	client, err := t.GetLobalClient(clientID)
	if err != nil {
		return
	}
	_ = client.Conn.CloseWithMsg([]byte("Client disconnected"))

	// 取消全部订阅
	// Cancel all subscriptions
	client.Unsub()

	// 从内存中移除
	// Remove from memory
	t.Clients.Delete(clientID)

	slog.Info("Websocket Del Client", slog.String("client", clientID))
}
