package websocket

// EventOnline 客户端在线
// EventOnline triggers when a client comes online
func (t *Service) EventOnline(clientId string) error {
	client, err := t.GetLobalClient(clientId)
	if err != nil {
		return err
	}

	err = t.Provider.Event("online", client)
	if err != nil {
		return err
	}

	return nil
}

// EventOffline 客户端离线
// EventOffline triggers when a client goes offline
func (t *Service) EventOffline(clientId string) error {
	client, err := t.GetLobalClient(clientId)
	if err != nil {
		return err
	}

	err = t.Provider.Event("offline", client)
	if err != nil {
		return err
	}
	return nil
}

// EventPing ping客户端
// EventPing triggers when a client sends a ping
func (t *Service) EventPing(clientId string) error {
	client, err := t.GetLobalClient(clientId)
	if err != nil {
		return err
	}

	err = t.Provider.Event("ping", client)
	if err != nil {
		return err
	}
	return nil
}
