package websocket

type SubscribeDriver interface {
	Subscribe(topic string, callback func(msg *Message)) error
	Publish(msg *Message) error
}
