package drivers

import (
	"context"
	"encoding/json"

	websocket "github.com/duxweb/go-ws"
	"github.com/spf13/cast"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
)

type MemorySubscribeDriver struct {
	pubSub *gochannel.GoChannel
}

func NewMemorySubscribeDriver() *MemorySubscribeDriver {
	pubSub := gochannel.NewGoChannel(
		gochannel.Config{},
		watermill.NewStdLogger(false, false),
	)
	return &MemorySubscribeDriver{
		pubSub: pubSub,
	}
}

func (d *MemorySubscribeDriver) Subscribe(topic string, callback func(msg *websocket.Message)) error {
	message, err := d.pubSub.Subscribe(context.Background(), topic)
	if err != nil {
		return err
	}
	go d.process(message, callback)
	return nil
}

func (d *MemorySubscribeDriver) Unsubscribe(topic string) error {
	return d.pubSub.Close()
}

func (d *MemorySubscribeDriver) Publish(msg *websocket.Message) error {
	payloadData := map[string]any{
		"message": msg.Message,
		"data":    msg.Data,
		"meta":    msg.Meta,
	}
	payload, _ := json.Marshal(payloadData)

	data := message.Message{
		UUID: msg.ID,
		Metadata: message.Metadata{
			"clientID": msg.Client,
			"channel":  msg.Channel,
			"type":     msg.Type,
		},
		Payload: payload,
	}
	return d.pubSub.Publish(msg.Channel, &data)
}

func (d *MemorySubscribeDriver) process(messages <-chan *message.Message, callback func(msg *websocket.Message)) {
	for msg := range messages {
		data := map[string]any{}
		json.Unmarshal(msg.Payload, &data)
		message := &websocket.Message{
			ID:      msg.UUID,
			Client:  msg.Metadata["clientID"],
			Channel: msg.Metadata["channel"],
			Type:    msg.Metadata["type"],
			Message: cast.ToString(data["message"]),
			Data:    data["data"],
			Meta:    data["meta"],
		}
		callback(message)
		msg.Ack()
	}
}
