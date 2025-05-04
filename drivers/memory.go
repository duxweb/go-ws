package drivers

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	websocket "github.com/duxweb/go-ws"
	"github.com/spf13/cast"
)

type MemoryDriver struct {
	// 订阅驱动
	// Subscribe driver
	pubSub *gochannel.GoChannel

	// 订阅主题
	// Subscribed topics
	// topicID -> context
	topics sync.Map

	// 频道-客户端映射
	// Topic-client mapping
	// topicID -> []string
	topicClients sync.Map

	// 客户端-频道映射
	// Client-topic mapping
	// clientID -> []string
	clientTopics sync.Map
}

func NewMemoryDriver() *MemoryDriver {
	pubSub := gochannel.NewGoChannel(
		gochannel.Config{},
		watermill.NewStdLogger(false, false),
	)
	return &MemoryDriver{
		pubSub: pubSub,
	}
}

func (d *MemoryDriver) Subscribe(topic string, callback func(msg *websocket.Message) error) error {

	ctx, cancel := context.WithCancel(context.Background())

	message, err := d.pubSub.Subscribe(ctx, topic)
	if err != nil {
		cancel()
		return err
	}

	d.topics.Store(topic, cancel)
	go d.process(message, callback)
	return nil
}

func (d *MemoryDriver) Unsubscribe(topic string) error {
	topicCtx, ok := d.topics.Load(topic)
	if !ok {
		return nil
	}

	if cancelFunc, ok := topicCtx.(context.CancelFunc); ok {
		cancelFunc()
	}

	d.topics.Delete(topic)
	return nil
}

func (d *MemoryDriver) Publish(msg *websocket.Message) error {
	payloadData := map[string]any{
		"message": msg.Message,
		"data":    msg.Data,
		"meta":    msg.Meta,
	}
	payload, _ := json.Marshal(payloadData)

	data := message.Message{
		UUID: msg.ID,
		Metadata: message.Metadata{
			"clientID": msg.ClientID,
			"channel":  msg.Channel,
			"type":     msg.Type,
		},
		Payload: payload,
	}
	return d.pubSub.Publish(msg.Channel, &data)
}

func (d *MemoryDriver) process(messages <-chan *message.Message, callback func(msg *websocket.Message) error) {
	for msg := range messages {
		data := map[string]any{}
		json.Unmarshal(msg.Payload, &data)
		message := &websocket.Message{
			ID:       msg.UUID,
			ClientID: msg.Metadata["clientID"],
			Channel:  msg.Metadata["channel"],
			Type:     msg.Metadata["type"],
			Message:  cast.ToString(data["message"]),
			Data:     data["data"],
			Meta:     data["meta"],
		}
		err := callback(message)
		if err != nil {
			slog.Error("Failed to process message", slog.Any("error", err))
			msg.Nack()
		} else {
			msg.Ack()
		}
	}
}

// CreateTopic 创建主题
// CreateTopic creates a topic
func (d *MemoryDriver) CreateTopic(topicID string) error {
	exists, _ := d.IsTopicExists(topicID)
	if !exists {
		// 只有在主题不存在时才创建
		// Only create if topic doesn't exist
		d.topicClients.Store(topicID, []string{})
	}
	return nil
}

// AddClientToTopic 将客户端添加到频道
// AddClientToTopic adds a client to a topic
func (d *MemoryDriver) AddClientToTopic(topicID string, clientID string) error {
	// 添加到频道-客户端映射
	// Add to topic-client mapping
	var clients []string
	if data, ok := d.topicClients.Load(topicID); ok {
		clients = data.([]string)
		// 检查客户端是否已在频道中
		// Check if client is already in the topic
		for _, id := range clients {
			if id == clientID {
				return nil // 客户端已在频道中，无需再添加
				// Client is already in the topic, no need to add again
			}
		}
	}
	clients = append(clients, clientID)
	d.topicClients.Store(topicID, clients)

	// 添加到客户端-频道映射
	// Add to client-topic mapping
	var topics []string
	if data, ok := d.clientTopics.Load(clientID); ok {
		topics = data.([]string)
		// 检查频道是否已被客户端订阅
		// Check if topic is already subscribed by the client
		for _, id := range topics {
			if id == topicID {
				return nil // 频道已被订阅，无需再添加
				// Topic is already subscribed, no need to add again
			}
		}
	}
	topics = append(topics, topicID)
	d.clientTopics.Store(clientID, topics)

	return nil
}

// RemoveClientFromTopic 从频道中移除客户端
// RemoveClientFromTopic removes a client from a topic
func (d *MemoryDriver) RemoveClientFromTopic(topicID string, clientID string) error {
	// 从频道-客户端映射中移除
	// Remove from topic-client mapping
	if data, ok := d.topicClients.Load(topicID); ok {
		clients := data.([]string)
		newClients := make([]string, 0, len(clients))
		for _, id := range clients {
			if id != clientID {
				newClients = append(newClients, id)
			}
		}
		if len(newClients) > 0 {
			d.topicClients.Store(topicID, newClients)
		} else {
			d.topicClients.Delete(topicID)
		}
	}

	// 从客户端-频道映射中移除
	// Remove from client-topic mapping
	if data, ok := d.clientTopics.Load(clientID); ok {
		topics := data.([]string)
		newTopics := make([]string, 0, len(topics))
		for _, id := range topics {
			if id != topicID {
				newTopics = append(newTopics, id)
			}
		}
		if len(newTopics) > 0 {
			d.clientTopics.Store(clientID, newTopics)
		} else {
			d.clientTopics.Delete(clientID)
		}
	}

	return nil
}

// IsTopicClient 检查客户端是否订阅主题
// IsTopicClient checks if a client is subscribed to a topic
func (d *MemoryDriver) IsTopicClient(topicID string, clientID string) (bool, error) {
	data, ok := d.topicClients.Load(topicID)
	if !ok {
		return false, nil
	}

	clients := data.([]string)
	for _, id := range clients {
		if id == clientID {
			return true, nil
		}
	}

	return false, nil
}

// GetTopicClients 获取频道中的所有客户端
// GetTopicClients gets all clients in a topic
func (d *MemoryDriver) GetTopicClients(topicID string) ([]string, error) {
	data, ok := d.topicClients.Load(topicID)
	if !ok {
		return []string{}, nil
	}
	return data.([]string), nil
}

// GetClientTopics 获取客户端订阅的所有频道
// GetClientTopics gets all topics subscribed by a client
func (d *MemoryDriver) GetClientTopics(clientID string) ([]string, error) {
	data, ok := d.clientTopics.Load(clientID)
	if !ok {
		return []string{}, nil
	}
	return data.([]string), nil
}

// IsTopicExists 检查频道是否存在
// IsTopicExists checks if a topic exists
func (d *MemoryDriver) IsTopicExists(topicID string) (bool, error) {
	_, ok := d.topicClients.Load(topicID)
	return ok, nil
}

// 确保 MemoryDriver 实现了 StorageDriver 接口
// Ensure MemoryDriver implements StorageDriver interface
var _ websocket.Driver = (*MemoryDriver)(nil)
