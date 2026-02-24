package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Hub manages all connected clients and the shared message history.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
	history []Message
}

// Client represents a single WebSocket connection to the hub.
type Client struct {
	name string
	conn *websocket.Conn
	send chan []byte
	hub  *Hub
}

// RunServer starts the WebSocket server on the given port.
func RunServer(port int) error {
	hub := &Hub{clients: make(map[string]*Client)}

	http.HandleFunc("/ws", hub.handleWS)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("claude-collab host listening on %s", addr)
	return http.ListenAndServe(addr, nil)
}

func (h *Hub) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	// First message must be a join within 5 seconds.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var join WireMsg
	if err := conn.ReadJSON(&join); err != nil || join.Type != "join" || join.Name == "" {
		conn.WriteJSON(WireMsg{Type: "error", Text: "expected join with name"})
		conn.Close()
		return
	}
	conn.SetReadDeadline(time.Time{}) // clear deadline

	client := &Client{
		name: join.Name,
		conn: conn,
		send: make(chan []byte, 64),
		hub:  h,
	}

	// Register client and send welcome.
	h.mu.Lock()
	h.clients[join.Name] = client
	peers := h.peerNames()
	historyCopy := make([]Message, len(h.history))
	copy(historyCopy, h.history)
	h.mu.Unlock()

	welcome := WireMsg{
		Type:    "welcome",
		Peers:   peers,
		History: historyCopy,
	}
	if err := conn.WriteJSON(welcome); err != nil {
		conn.Close()
		return
	}

	// Broadcast peer_joined to everyone else.
	h.broadcast(WireMsg{Type: "peer_joined", Name: join.Name}, join.Name)

	log.Printf("peer joined: %s", join.Name)

	go client.writePump()
	client.readPump()
}

// peerNames returns all connected client names. Must be called with h.mu held.
func (h *Hub) peerNames() []string {
	names := make([]string, 0, len(h.clients))
	for name := range h.clients {
		names = append(names, name)
	}
	return names
}

// broadcast sends a message to all clients except the excluded name.
func (h *Hub) broadcast(msg WireMsg, exclude string) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for name, client := range h.clients {
		if name == exclude {
			continue
		}
		select {
		case client.send <- data:
		default:
			// drop message if client buffer is full
		}
	}
}

// broadcastAll sends a message to all clients including sender.
func (h *Hub) broadcastAll(msg WireMsg) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		select {
		case client.send <- data:
		default:
		}
	}
}

// removeClient unregisters a client and broadcasts peer_left.
func (h *Hub) removeClient(name string) {
	h.mu.Lock()
	delete(h.clients, name)
	h.mu.Unlock()

	h.broadcastAll(WireMsg{Type: "peer_left", Name: name})
	log.Printf("peer left: %s", name)
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.hub.removeClient(c.name)
		c.conn.Close()
	}()

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var msg WireMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		if msg.Type != "message" {
			continue
		}

		broadcast := WireMsg{
			Type: "message",
			From: c.name,
			Text: msg.Text,
			Ts:   nowTS(),
		}

		c.hub.mu.Lock()
		c.hub.history = append(c.hub.history, Message{
			From: broadcast.From,
			Text: broadcast.Text,
			Ts:   broadcast.Ts,
		})
		c.hub.mu.Unlock()

		c.hub.broadcastAll(broadcast)
	}
}

// writePump sends queued messages to the WebSocket connection.
func (c *Client) writePump() {
	defer c.conn.Close()

	for data := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}
	}
}
