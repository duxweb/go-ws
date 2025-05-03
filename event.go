package websocket

// EventOnline 客户端在线
// EventOnline triggers when a client comes online
func (t *ServiceT) EventOnline(clientId string) error {
	client, err := t.GetClient(clientId)
	if err != nil {
		return err
	}

	provider := t.Providers[client.App]
	err = provider.Event("online", client)
	if err != nil {
		return err
	}

	return nil
}

// EventOffline 客户端离线
// EventOffline triggers when a client goes offline
func (t *ServiceT) EventOffline(clientId string) error {
	client, err := t.GetClient(clientId)
	if err != nil {
		return err
	}

	provider := t.Providers[client.App]
	err = provider.Event("offline", client)
	if err != nil {
		return err
	}
	return nil
}

// EventPing ping客户端
// EventPing triggers when a client sends a ping
func (t *ServiceT) EventPing(clientId string) error {
	client, err := t.GetClient(clientId)
	if err != nil {
		return err
	}

	provider := t.Providers[client.App]
	err = provider.Event("ping", client)
	if err != nil {
		return err
	}
	return nil
}
