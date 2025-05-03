package websocket

import (
	"errors"
	"log/slog"

	"github.com/spf13/cast"
)

// Sub 订阅频道
// Sub subscribes to channels
func (p *Client) Sub(channels ...string) error {
	if len(channels) < 1 {
		return errors.New("empty channel")
	}

	slog.Info("Websocket Client Sub", slog.String("client", p.ClientID), slog.Any("channels", channels))

	for _, topic := range channels {
		// 判断频道是否存在
		// Check if channel exists
		if !isExists(p.Channel, topic) {
			p.Channel.PushBack(topic)
			// 订阅频道
			// Subscribe to channel
			p.Service.Driver.Subscribe(topic, func(msg *Message) {
				p.Service.SendClient(p.ClientID, msg)
			})
		}

		// 判断客户是否订阅该频道
		// Check if client has already subscribed to this channel
		if data, ok := p.Service.Channels.Load(topic); ok {
			item := data.(*Channel)
			// 判断频道是否包含客户
			// Check if channel contains this client
			if !isExists(item.Channel, p.ClientID) {
				item.Channel.PushFront(p.ClientID)
			}
		} else {
			// 创建新的频道映射
			// Create new channel mapping
			clients := listLock{}
			clients.PushBack(p.ClientID)
			p.Service.Channels.Store(topic, &Channel{
				ClientID: topic,
				Channel:  &clients,
			})
		}
	}
	return nil
}

// Unsub 取消订阅频道
// Unsub unsubscribes from channels
func (p *Client) Unsub(args ...string) {

	var topics = make([]string, 0)
	if len(args) <= 0 {
		for e := p.Channel.Front(); e != nil; e = e.Next() {
			topics = append(topics, cast.ToString(e.Value))
		}
	} else {
		topics = args
	}

	slog.Info("Websocket Client Unsub", slog.String("client", p.ClientID), slog.Any("topics", topics))

	for _, topic := range topics {
		// 判断频道是否存在
		// Check if channel exists
		data, ok := p.Service.Channels.Load(topic)
		if !ok {
			continue
		}

		// 从频道中移除客户端
		// Remove client from channel
		item := data.(*Channel)
		for e := item.Channel.Front(); e != nil; e = e.Next() {
			if cast.ToString(e.Value) == p.ClientID {
				item.Channel.Remove(e)
			}
		}

		// 如果频道为空则删除频道
		// Delete channel if it's empty
		if item.Channel.Len() == 0 {
			p.Service.Channels.Delete(topic)
		}

		// 从客户端中移除频道
		// Remove channel from client
		for e := p.Channel.Front(); e != nil; e = e.Next() {
			if cast.ToString(e.Value) == topic {
				p.Channel.Remove(e)
			}
		}
	}
}

// Send 发布消息
// Send publishes a message to client
func (p *Client) Send(data *Message) {
	err := p.Service.SendClient(p.ClientID, data)
	slog.Error("Websocket Send", slog.Any("error", err), slog.String("client", p.ClientID), slog.Any("data", data))
}

// Push 发布频道消息
// Push publishes a message to channels
func (p *Client) Push(channels []string, data *Message) {
	for _, topic := range channels {
		msg := &Message{
			ID:      data.ID,
			Type:    data.Type,
			Client:  data.Client,
			Channel: topic,
			Message: data.Message,
			Data:    data.Data,
			Meta:    data.Meta,
		}
		p.Service.Driver.Publish(msg)
	}
}

// isExists 检查值是否存在于列表中
// isExists checks if a value exists in the list
func isExists(list *listLock, value string) bool {
	for e := list.Front(); e != nil; e = e.Next() {
		if cast.ToString(e.Value) == value {
			return true
		}
	}
	return false
}
