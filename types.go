package websocket

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

// Driver 驱动接口
// Driver interface
type Driver interface {
	// Subscribe 订阅主题
	// Subscribe subscribes to a topic
	Subscribe(topic string, callback func(msg *Message) error) error

	// Unsubscribe 取消订阅主题
	// Unsubscribe unsubscribes from a topic
	Unsubscribe(topic string) error

	// Publish 发布消息
	// Publish publishes a message
	Publish(msg *Message) error

	// CreateTopic 创建主题
	// CreateTopic creates a topic
	CreateTopic(topicID string) error

	// IsTopicExists 检查主题是否存在
	// IsTopicExists checks if a topic exists
	IsTopicExists(topicID string) (bool, error)

	// AddClientToTopic 将客户端添加到主题
	// AddClientToTopic adds a client to a topic
	AddClientToTopic(topicID string, clientID string) error

	// RemoveClientFromTopic 从主题中移除客户端
	// RemoveClientFromTopic removes a client from a topic
	RemoveClientFromTopic(topicID string, clientID string) error

	// IsTopicClient 检查客户端是否订阅主题
	// IsTopicClient checks if a client is subscribed to a topic
	IsTopicClient(topicID string, clientID string) (bool, error)

	// GetTopicClients 获取主题中的所有客户端
	// GetTopicClients gets all clients in a topic
	GetTopicClients(topicID string) ([]string, error)

	// GetClientTopics 获取客户端订阅的所有主题
	// GetClientTopics gets all topics subscribed by a client
	GetClientTopics(clientID string) ([]string, error)
}
