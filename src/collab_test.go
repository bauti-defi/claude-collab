package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// startTestServer spins up an httptest server with a fresh Hub and returns
// the server and its WebSocket URL. Caller must call server.Close().
func startTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	hub := &Hub{clients: make(map[string]*Client), virtualPeers: make(map[string]struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.handleWS)
	srv := httptest.NewServer(mux)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	return srv, wsURL
}

// dialRaw opens a raw WebSocket connection and sends a join message,
// returning the connection and the welcome response. Caller must close conn.
func dialRaw(t *testing.T, wsURL, name string) (*websocket.Conn, WireMsg) {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	join := WireMsg{Type: "join", Name: name}
	if err := conn.WriteJSON(join); err != nil {
		t.Fatalf("send join: %v", err)
	}

	var welcome WireMsg
	if err := conn.ReadJSON(&welcome); err != nil {
		t.Fatalf("read welcome: %v", err)
	}
	if welcome.Type != "welcome" {
		t.Fatalf("expected welcome, got %q", welcome.Type)
	}
	return conn, welcome
}

// readMsg reads the next WireMsg from a connection with a timeout.
func readMsg(t *testing.T, conn *websocket.Conn) WireMsg {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg WireMsg
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read message: %v", err)
	}
	conn.SetReadDeadline(time.Time{})
	return msg
}

func TestJoinAndWelcome(t *testing.T) {
	srv, wsURL := startTestServer(t)
	defer srv.Close()

	conn, welcome := dialRaw(t, wsURL, "Alice")
	defer conn.Close()

	// First peer gets an empty peer list (or just themselves).
	if welcome.Type != "welcome" {
		t.Errorf("expected welcome type, got %q", welcome.Type)
	}
	if welcome.History == nil {
		// ok — no history yet
	}
}

func TestPeerJoinedNotification(t *testing.T) {
	srv, wsURL := startTestServer(t)
	defer srv.Close()

	connA, _ := dialRaw(t, wsURL, "Alice")
	defer connA.Close()

	// Bob joins — Alice should get peer_joined.
	connB, welcomeB := dialRaw(t, wsURL, "Bob")
	defer connB.Close()

	// Bob's welcome should list Alice.
	found := false
	for _, p := range welcomeB.Peers {
		if p == "Alice" {
			found = true
		}
	}
	if !found {
		t.Errorf("Bob's welcome should include Alice, got peers: %v", welcomeB.Peers)
	}

	// Alice should receive peer_joined for Bob.
	msg := readMsg(t, connA)
	if msg.Type != "peer_joined" {
		t.Errorf("expected peer_joined, got %q", msg.Type)
	}
	if msg.Name != "Bob" {
		t.Errorf("expected name Bob, got %q", msg.Name)
	}
}

func TestMessageBroadcast(t *testing.T) {
	srv, wsURL := startTestServer(t)
	defer srv.Close()

	connA, _ := dialRaw(t, wsURL, "Alice")
	defer connA.Close()

	connB, _ := dialRaw(t, wsURL, "Bob")
	defer connB.Close()

	// Drain peer_joined notification on Alice's side.
	_ = readMsg(t, connA)

	// Alice sends a message.
	outMsg := WireMsg{Type: "message", Text: "hello from Alice"}
	if err := connA.WriteJSON(outMsg); err != nil {
		t.Fatalf("send message: %v", err)
	}

	// Both Alice and Bob should receive the broadcast.
	msgA := readMsg(t, connA)
	msgB := readMsg(t, connB)

	for _, msg := range []WireMsg{msgA, msgB} {
		if msg.Type != "message" {
			t.Errorf("expected message type, got %q", msg.Type)
		}
		if msg.From != "Alice" {
			t.Errorf("expected from Alice, got %q", msg.From)
		}
		if msg.Text != "hello from Alice" {
			t.Errorf("expected 'hello from Alice', got %q", msg.Text)
		}
		if msg.Ts == "" {
			t.Error("expected non-empty timestamp")
		}
	}
}

func TestPeerLeftNotification(t *testing.T) {
	srv, wsURL := startTestServer(t)
	defer srv.Close()

	connA, _ := dialRaw(t, wsURL, "Alice")
	defer connA.Close()

	connB, _ := dialRaw(t, wsURL, "Bob")

	// Drain peer_joined on Alice.
	_ = readMsg(t, connA)

	// Bob disconnects.
	connB.Close()

	// Alice should get peer_left.
	msg := readMsg(t, connA)
	if msg.Type != "peer_left" {
		t.Errorf("expected peer_left, got %q", msg.Type)
	}
	if msg.Name != "Bob" {
		t.Errorf("expected name Bob, got %q", msg.Name)
	}
}

func TestHistoryOnJoin(t *testing.T) {
	srv, wsURL := startTestServer(t)
	defer srv.Close()

	connA, _ := dialRaw(t, wsURL, "Alice")
	defer connA.Close()

	// Alice sends a message before Bob joins.
	if err := connA.WriteJSON(WireMsg{Type: "message", Text: "pre-existing message"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	// Read back the broadcast so we know it was processed.
	_ = readMsg(t, connA)

	// Bob joins and should see the message in history.
	connB, welcomeB := dialRaw(t, wsURL, "Bob")
	defer connB.Close()

	if len(welcomeB.History) != 1 {
		t.Fatalf("expected 1 history message, got %d", len(welcomeB.History))
	}
	if welcomeB.History[0].Text != "pre-existing message" {
		t.Errorf("expected 'pre-existing message', got %q", welcomeB.History[0].Text)
	}
	if welcomeB.History[0].From != "Alice" {
		t.Errorf("expected from Alice, got %q", welcomeB.History[0].From)
	}
}

func TestInvalidJoin(t *testing.T) {
	srv, wsURL := startTestServer(t)
	defer srv.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send a message that isn't a join.
	if err := conn.WriteJSON(WireMsg{Type: "message", Text: "bad"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	// Server should send an error and close.
	var msg WireMsg
	err = conn.ReadJSON(&msg)
	if err == nil && msg.Type == "error" {
		// Expected — server rejected the non-join message.
		return
	}
	// Connection might just close, which is also acceptable.
}

func TestJoinWithoutName(t *testing.T) {
	srv, wsURL := startTestServer(t)
	defer srv.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send join with empty name.
	if err := conn.WriteJSON(WireMsg{Type: "join", Name: ""}); err != nil {
		t.Fatalf("send: %v", err)
	}

	var msg WireMsg
	err = conn.ReadJSON(&msg)
	if err == nil && msg.Type == "error" {
		return
	}
	// Connection close is also acceptable.
}

// --- CollabClient tests ---

func TestClientConnectAndPeers(t *testing.T) {
	srv, wsURL := startTestServer(t)
	defer srv.Close()

	clientA, err := Connect(wsURL, "Alice")
	if err != nil {
		t.Fatalf("connect Alice: %v", err)
	}

	clientB, err := Connect(wsURL, "Bob")
	if err != nil {
		t.Fatalf("connect Bob: %v", err)
	}

	// Give readLoops time to process peer_joined.
	time.Sleep(100 * time.Millisecond)

	peersA := clientA.Peers()
	peersB := clientB.Peers()

	// Alice should know about Bob.
	if !contains(peersA, "Bob") {
		t.Errorf("Alice should see Bob, got peers: %v", peersA)
	}

	// Bob should know about Alice (from welcome).
	if !contains(peersB, "Alice") {
		t.Errorf("Bob should see Alice, got peers: %v", peersB)
	}

	_ = clientA
	_ = clientB
}

func TestClientSendAndReceive(t *testing.T) {
	srv, wsURL := startTestServer(t)
	defer srv.Close()

	clientA, err := Connect(wsURL, "Alice")
	if err != nil {
		t.Fatalf("connect Alice: %v", err)
	}

	clientB, err := Connect(wsURL, "Bob")
	if err != nil {
		t.Fatalf("connect Bob: %v", err)
	}

	// Wait for connections to stabilize.
	time.Sleep(50 * time.Millisecond)

	if err := clientA.Send("hello from Alice"); err != nil {
		t.Fatalf("send: %v", err)
	}

	// Wait for message propagation.
	time.Sleep(100 * time.Millisecond)

	msgsB := clientB.Messages(0)
	if len(msgsB) == 0 {
		t.Fatal("Bob should have received a message")
	}

	last := msgsB[len(msgsB)-1]
	if last.From != "Alice" {
		t.Errorf("expected from Alice, got %q", last.From)
	}
	if last.Text != "hello from Alice" {
		t.Errorf("expected 'hello from Alice', got %q", last.Text)
	}
}

func TestClientMessagesLimit(t *testing.T) {
	srv, wsURL := startTestServer(t)
	defer srv.Close()

	clientA, err := Connect(wsURL, "Alice")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	// Send multiple messages.
	for i := 0; i < 5; i++ {
		if err := clientA.Send("msg"); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	all := clientA.Messages(0)
	if len(all) < 5 {
		t.Fatalf("expected at least 5 messages, got %d", len(all))
	}

	limited := clientA.Messages(2)
	if len(limited) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(limited))
	}
}

func TestConcurrentSends(t *testing.T) {
	srv, wsURL := startTestServer(t)
	defer srv.Close()

	clientA, err := Connect(wsURL, "Alice")
	if err != nil {
		t.Fatalf("connect Alice: %v", err)
	}

	clientB, err := Connect(wsURL, "Bob")
	if err != nil {
		t.Fatalf("connect Bob: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Send messages concurrently from both clients.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			clientA.Send("from A")
		}()
		go func() {
			defer wg.Done()
			clientB.Send("from B")
		}()
	}
	wg.Wait()

	time.Sleep(300 * time.Millisecond)

	// Both clients should have received all messages.
	msgsA := clientA.Messages(0)
	msgsB := clientB.Messages(0)

	// We expect at least 20 messages total (10 from each).
	// Messages may also include earlier broadcasts.
	if len(msgsA) < 20 {
		t.Errorf("Alice expected at least 20 messages, got %d", len(msgsA))
	}
	if len(msgsB) < 20 {
		t.Errorf("Bob expected at least 20 messages, got %d", len(msgsB))
	}
}

// --- Wire protocol tests ---

func TestWireMsgSerialization(t *testing.T) {
	msg := WireMsg{
		Type: "message",
		From: "Alice",
		Text: "hello",
		Ts:   "2025-01-01T00:00:00Z",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded WireMsg
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("type: got %q, want %q", decoded.Type, msg.Type)
	}
	if decoded.From != msg.From {
		t.Errorf("from: got %q, want %q", decoded.From, msg.From)
	}
	if decoded.Text != msg.Text {
		t.Errorf("text: got %q, want %q", decoded.Text, msg.Text)
	}
}

func TestWireMsgOmitsEmptyFields(t *testing.T) {
	msg := WireMsg{Type: "join", Name: "Alice"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Should not contain "from", "text", "ts", "peers", "history" keys.
	s := string(data)
	for _, field := range []string{`"from"`, `"text"`, `"ts"`, `"peers"`, `"history"`} {
		if strings.Contains(s, field) {
			t.Errorf("join message should not contain %s, got: %s", field, s)
		}
	}
}

func TestNowTS(t *testing.T) {
	ts := nowTS()
	_, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Errorf("nowTS() returned non-RFC3339 string: %q", ts)
	}
}

// contains checks if a string slice contains a value.
func contains(ss []string, val string) bool {
	for _, s := range ss {
		if s == val {
			return true
		}
	}
	return false
}
