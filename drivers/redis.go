package drivers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	websocket "github.com/duxweb/go-ws"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
)

type RedisDriver struct {
	// 客户端
	// Client
	client *redis.Client
	// 发布驱动
	// Publish driver
	publisher *redisstream.Publisher

	// 订阅驱动
	// Subscribe driver
	subscriber *redisstream.Subscriber
	// 订阅主题
	// Subscribed topics
	topics sync.Map

	// Redis键前缀
	// Redis key prefix
	keyPrefix string
}

// RedisOptions Redis锁配置选项
// RedisOptions Redis lock configuration options
type RedisOptions struct {
	Client       *redis.Client
	Addr         string
	Password     string
	DB           int
	Prefix       string
	PoolSize     int
	MinIdleConns int
}

func NewRedisDriver(opts *RedisOptions) (*RedisDriver, error) {

	var client *redis.Client

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if opts.Client != nil {
		client = opts.Client
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:         opts.Addr,
			Password:     opts.Password,
			DB:           opts.DB,
			PoolSize:     opts.PoolSize,
			MinIdleConns: opts.MinIdleConns,
		})
	}

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis: %w", err)
	}

	publisher, err := redisstream.NewPublisher(
		redisstream.PublisherConfig{
			Client:     client,
			Marshaller: redisstream.DefaultMarshallerUnmarshaller{},
		},
		watermill.NewSlogLogger(nil),
	)

	if err != nil {
		return nil, fmt.Errorf("Redis: %w", err)
	}

	subscriber, err := redisstream.NewSubscriber(
		redisstream.SubscriberConfig{
			Client:       client,
			Unmarshaller: redisstream.DefaultMarshallerUnmarshaller{},
		},
		watermill.NewSlogLogger(nil),
	)

	if err != nil {
		return nil, fmt.Errorf("Redis: %w", err)
	}

	prefix := "ws:"
	if opts.Prefix != "" {
		prefix = opts.Prefix + ":"
	}

	return &RedisDriver{
		publisher:  publisher,
		subscriber: subscriber,
		client:     client,
		keyPrefix:  prefix,
	}, nil
}

func (d *RedisDriver) Subscribe(topic string, callback func(msg *websocket.Message) error) error {
	ctx, cancel := context.WithCancel(context.Background())
	message, err := d.subscriber.Subscribe(ctx, topic)
	if err != nil {
		cancel()
		return err
	}

	d.topics.Store(topic, cancel)
	go d.process(message, callback)
	return nil
}

func (d *RedisDriver) Unsubscribe(topic string) error {
	topicCtxValue, ok := d.topics.Load(topic)
	if !ok {
		return nil
	}

	if cancelFunc, ok := topicCtxValue.(context.CancelFunc); ok {
		cancelFunc()
	}

	d.topics.Delete(topic)

	return nil
}

func (d *RedisDriver) Publish(msg *websocket.Message) error {
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
	return d.publisher.Publish(msg.Channel, &data)
}

func (d *RedisDriver) process(messages <-chan *message.Message, callback func(msg *websocket.Message) error) {
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
func (d *RedisDriver) CreateTopic(topicID string) error {
	ctx := context.Background()
	exists, _ := d.IsTopicExists(topicID)
	if !exists {
		return d.client.Set(ctx, d.topicKey(topicID), 1, 0).Err()
	}
	return nil
}

// IsTopicExists 检查频道是否存在
// IsTopicExists checks if a topic exists
func (d *RedisDriver) IsTopicExists(topicID string) (bool, error) {
	ctx := context.Background()
	exists, err := d.client.Exists(ctx, d.topicKey(topicID)).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// AddClientToTopic 将客户端添加到频道
// AddClientToTopic adds a client to a topic
func (d *RedisDriver) AddClientToTopic(topicID string, clientID string) error {
	ctx := context.Background()

	if err := d.CreateTopic(topicID); err != nil {
		return err
	}

	exists, err := d.IsTopicClient(topicID, clientID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	if err := d.client.SAdd(ctx, d.topicClientsKey(topicID), clientID).Err(); err != nil {
		return err
	}

	if err := d.client.SAdd(ctx, d.clientTopicsKey(clientID), topicID).Err(); err != nil {
		return err
	}

	return nil
}

// RemoveClientFromTopic 从频道中移除客户端
// RemoveClientFromTopic removes a client from a topic
func (d *RedisDriver) RemoveClientFromTopic(topicID string, clientID string) error {
	ctx := context.Background()

	if err := d.client.SRem(ctx, d.topicClientsKey(topicID), clientID).Err(); err != nil {
		return err
	}

	if err := d.client.SRem(ctx, d.clientTopicsKey(clientID), topicID).Err(); err != nil {
		return err
	}

	count, err := d.client.SCard(ctx, d.topicClientsKey(topicID)).Result()
	if err != nil {
		return err
	}

	if count == 0 {
		if err := d.client.Del(ctx, d.topicClientsKey(topicID)).Err(); err != nil {
			return err
		}
		if err := d.client.Del(ctx, d.topicKey(topicID)).Err(); err != nil {
			return err
		}
	}

	return nil
}

// IsTopicClient 检查客户端是否订阅主题
// IsTopicClient checks if a client is subscribed to a topic
func (d *RedisDriver) IsTopicClient(topicID string, clientID string) (bool, error) {
	ctx := context.Background()
	exists, err := d.client.SIsMember(ctx, d.topicClientsKey(topicID), clientID).Result()
	if err != nil {
		return false, err
	}
	return exists, nil
}

// GetTopicClients 获取频道中的所有客户端
// GetTopicClients gets all clients in a topic
func (d *RedisDriver) GetTopicClients(topicID string) ([]string, error) {
	ctx := context.Background()
	clients, err := d.client.SMembers(ctx, d.topicClientsKey(topicID)).Result()
	if err != nil {
		return nil, err
	}
	return clients, nil
}

// GetClientTopics 获取客户端订阅的所有频道
// GetClientTopics gets all topics subscribed by a client
func (d *RedisDriver) GetClientTopics(clientID string) ([]string, error) {
	ctx := context.Background()
	topics, err := d.client.SMembers(ctx, d.clientTopicsKey(clientID)).Result()
	if err != nil {
		return nil, err
	}
	return topics, nil
}

// 辅助方法：生成主题-客户端集合键
// Helper method: generate topic-client set key
func (d *RedisDriver) topicClientsKey(topicID string) string {
	return d.keyPrefix + "topic:" + topicID + ":clients"
}

// 辅助方法：生成客户端-主题集合键
// Helper method: generate client-topic set key
func (d *RedisDriver) clientTopicsKey(clientID string) string {
	return d.keyPrefix + "client:" + clientID + ":topics"
}

// 辅助方法：生成主题键
// Helper method: generate topic key
func (d *RedisDriver) topicKey(topicID string) string {
	return d.keyPrefix + "topic:" + topicID
}

// 确保 RedisDriver 实现了 StorageDriver 接口
// Ensure RedisDriver implements StorageDriver interface
var _ websocket.Driver = (*RedisDriver)(nil)
