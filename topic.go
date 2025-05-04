package websocket

import (
	"log/slog"
	"sync"
)

// SubTopic 订阅主题
// SubTopic subscribes to a topic
func (s *Service) SubTopic(topic string) error {
	exists, err := s.Driver.IsTopicExists(topic)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	// 创建主题
	// Create topic
	err = s.Driver.CreateTopic(topic)
	if err != nil {
		return err
	}

	// 订阅主题
	// Subscribe to topic
	err = s.Driver.Subscribe(topic, func(msg *Message) error {
		clients, err := s.Driver.GetTopicClients(topic)
		if err != nil {
			slog.Error("Failed to get topic clients", slog.Any("error", err))
			return err
		}
		wg := sync.WaitGroup{}
		for _, clientID := range clients {
			if !s.IsLocalClient(clientID) {
				continue
			}
			wg.Add(1)
			go func(clientID string) {
				defer wg.Done()
				s.SendLocalClient(clientID, msg)
			}(clientID)
		}
		wg.Wait()
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

// ClientToTopic 客户端订阅主题
// ClientToTopic subscribes a client to a topic
func (s *Service) ClientToTopic(clientID string, topicID string) error {
	exists, err := s.Driver.IsTopicClient(topicID, clientID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.Driver.AddClientToTopic(topicID, clientID)
}

// UnsubClientFromTopic 取消客户端订阅主题
// UnsubClientFromTopic unsubscribes a client from a topic
func (s *Service) UnsubClientFromTopic(clientID string, topicID string) error {
	return s.Driver.RemoveClientFromTopic(topicID, clientID)
}

// GetTopicClients 获取主题的客户端
// GetTopicClients gets the clients of a topic
func (s *Service) GetTopicClients(topicID string) ([]string, error) {
	return s.Driver.GetTopicClients(topicID)
}

// GetClientTopics 获取客户端订阅的主题
// GetClientTopics gets the topics subscribed by a client
func (s *Service) GetClientTopics(clientID string) ([]string, error) {
	return s.Driver.GetClientTopics(clientID)
}

// UnsubTopic 取消订阅主题
// UnsubTopic unsubscribes a topic
func (s *Service) UnsubTopic(topicID string) error {
	return s.Driver.Unsubscribe(topicID)
}
