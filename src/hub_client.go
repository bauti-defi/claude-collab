package main

// HubClient provides direct in-process access to a Hub, implementing the
// Collaborator interface without needing a WebSocket connection. Used by
// the host's remote MCP endpoint so claude.ai can participate in the chat.
type HubClient struct {
	hub  *Hub
	name string
}

func NewHubClient(hub *Hub, name string) *HubClient {
	hc := &HubClient{hub: hub, name: name}

	// Register as a virtual peer so we show up in peer lists.
	hub.mu.Lock()
	hub.virtualPeers[name] = struct{}{}
	hub.mu.Unlock()

	// Announce join to WebSocket clients.
	hub.broadcastAll(WireMsg{Type: "peer_joined", Name: name})

	return hc
}

func (hc *HubClient) Send(text string) error {
	msg := WireMsg{Type: "message", From: hc.name, Text: text, Ts: nowTS()}

	hc.hub.mu.Lock()
	hc.hub.history = append(hc.hub.history, Message{
		From: msg.From,
		Text: msg.Text,
		Ts:   msg.Ts,
	})
	hc.hub.mu.Unlock()

	hc.hub.broadcastAll(msg)
	return nil
}

func (hc *HubClient) Messages(limit int) []Message {
	hc.hub.mu.RLock()
	defer hc.hub.mu.RUnlock()

	msgs := hc.hub.history
	if limit <= 0 || limit > len(msgs) {
		limit = len(msgs)
	}

	start := len(msgs) - limit
	out := make([]Message, limit)
	copy(out, msgs[start:])
	return out
}

func (hc *HubClient) Peers() []string {
	hc.hub.mu.RLock()
	defer hc.hub.mu.RUnlock()
	return hc.hub.peerNames()
}

func (hc *HubClient) PeerCount() int {
	hc.hub.mu.RLock()
	defer hc.hub.mu.RUnlock()
	return len(hc.hub.clients) + len(hc.hub.virtualPeers)
}
