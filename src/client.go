package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Collaborator is the interface for sending and reading collab messages.
// Both CollabClient (WebSocket) and HubClient (in-process) implement it.
type Collaborator interface {
	Send(text string) error
	Messages(limit int) []Message
	Peers() []string
	PeerCount() int
}

// Wire protocol types shared between server and client.

type WireMsg struct {
	Type    string    `json:"type"`
	Name    string    `json:"name,omitempty"`
	From    string    `json:"from,omitempty"`
	Text    string    `json:"text,omitempty"`
	Ts      string    `json:"ts,omitempty"`
	Peers   []string  `json:"peers,omitempty"`
	History []Message `json:"history,omitempty"`
}

type Message struct {
	From string `json:"from"`
	Text string `json:"text"`
	Ts   string `json:"ts"`
}

// CollabClient manages the WebSocket connection to the collab server.
type CollabClient struct {
	conn    *websocket.Conn
	name    string
	writeMu sync.Mutex   // protects conn writes (gorilla/websocket is not concurrent-write-safe)
	mu      sync.RWMutex // protects peers and messages
	peers    []string
	messages []Message
}

// Connect dials the WebSocket server, sends a join message, and reads the welcome response.
// It starts a background readLoop to process incoming messages.
func Connect(url, name string) (*CollabClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", url, err)
	}

	join := WireMsg{Type: "join", Name: name}
	if err := conn.WriteJSON(join); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send join: %w", err)
	}

	var welcome WireMsg
	if err := conn.ReadJSON(&welcome); err != nil {
		conn.Close()
		return nil, fmt.Errorf("read welcome: %w", err)
	}
	if welcome.Type != "welcome" {
		conn.Close()
		return nil, fmt.Errorf("expected welcome, got %q", welcome.Type)
	}

	c := &CollabClient{
		conn:     conn,
		name:     name,
		peers:    welcome.Peers,
		messages: welcome.History,
	}

	go c.readLoop()
	return c, nil
}

// readLoop processes incoming messages from the server in a background goroutine.
func (c *CollabClient) readLoop() {
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return // connection closed
		}

		var msg WireMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		c.mu.Lock()
		switch msg.Type {
		case "message":
			c.messages = append(c.messages, Message{
				From: msg.From,
				Text: msg.Text,
				Ts:   msg.Ts,
			})
		case "peer_joined":
			c.peers = append(c.peers, msg.Name)
		case "peer_left":
			filtered := c.peers[:0]
			for _, p := range c.peers {
				if p != msg.Name {
					filtered = append(filtered, p)
				}
			}
			c.peers = filtered
		}
		c.mu.Unlock()
	}
}

// Send broadcasts a text message to all peers via the server.
func (c *CollabClient) Send(text string) error {
	msg := WireMsg{Type: "message", Text: text}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteJSON(msg)
}

// Messages returns the last `limit` messages. If limit <= 0, returns all.
func (c *CollabClient) Messages(limit int) []Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 || limit > len(c.messages) {
		limit = len(c.messages)
	}

	start := len(c.messages) - limit
	out := make([]Message, limit)
	copy(out, c.messages[start:])
	return out
}

// Peers returns the current list of connected peer names (excluding self).
func (c *CollabClient) Peers() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]string, len(c.peers))
	copy(out, c.peers)
	return out
}

// PeerCount returns the number of currently connected peers.
func (c *CollabClient) PeerCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.peers)
}

// nowTS returns an RFC3339 timestamp for the current time.
func nowTS() string {
	return time.Now().UTC().Format(time.RFC3339)
}
