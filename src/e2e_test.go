package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestFullConversation simulates a multi-turn conversation between three
// Claude Code sessions collaborating on a task over the collab channel.
func TestFullConversation(t *testing.T) {
	// Start server
	hub := &Hub{clients: make(map[string]*Client)}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.handleWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	// Connect three Claude sessions
	bauti, err := Connect(wsURL, "Claudio-Bauti")
	if err != nil {
		t.Fatalf("connect Bauti: %v", err)
	}
	pablo, err := Connect(wsURL, "Claudio-Pablo")
	if err != nil {
		t.Fatalf("connect Pablo: %v", err)
	}
	chasco, err := Connect(wsURL, "Claudio-Chasco")
	if err != nil {
		t.Fatalf("connect Chasco: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// === The conversation ===
	conversation := []struct {
		sender  *CollabClient
		name    string
		message string
	}{
		{bauti, "Claudio-Bauti", "STATUS: Starting work on the Aerodrome integration in Our-DeFi-Kit. Will be modifying src/protocols/aerodrome/"},
		{pablo, "Claudio-Pablo", "STATUS: Working on the lending strategy backtest in DAMM-analytics. Won't touch any shared code."},
		{chasco, "Claudio-Chasco", "STATUS: Debugging the Telegram bot alert formatting. Working in DAMM-telegram-bot/src/format/"},
		{bauti, "Claudio-Bauti", "QUESTION: @Pablo — do we have Aerodrome pool data indexed in damm-public yet? I need TVL snapshots."},
		{pablo, "Claudio-Pablo", "Yes, aerodrome_pool_snapshots table exists in damm-public. Indexed hourly since last month. Use the pool_address + timestamp columns."},
		{bauti, "Claudio-Bauti", "Perfect, thanks. That saves me a pipeline task."},
		{chasco, "Claudio-Chasco", "HEADS-UP: I'm about to update the shared message formatting utils in sdk/src/format.ts. If either of you import from there, hold off on pulls for ~10 min."},
		{bauti, "Claudio-Bauti", "I don't import from sdk format — you're good."},
		{pablo, "Claudio-Pablo", "Same, all Python on my end. Go for it."},
		{chasco, "Claudio-Chasco", "DONE: SDK format utils updated. Safe to pull."},
		{pablo, "Claudio-Pablo", "DONE: Backtest framework v1 complete. 3 strategies tested, results in DAMM-analytics/pablo/results/lending-backtest-2026-02.csv"},
		{bauti, "Claudio-Bauti", "DONE: Aerodrome Permit2 permissions added to Our-DeFi-Kit. PR ready for review: bauti/aerodrome-permit2"},
		{chasco, "Claudio-Chasco", "DONE: Telegram alert formatting fixed. Bot now shows proper USD values and token symbols. PR: bauti/tg-format-fix"},
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║           claude-collab: Live Conversation Demo             ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println()

	for i, turn := range conversation {
		if err := turn.sender.Send(turn.message); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
		time.Sleep(50 * time.Millisecond) // let it propagate

		fmt.Printf("  [%s]: %s\n", turn.name, turn.message)

		if i == 2 || i == 5 || i == 9 {
			fmt.Println("  ─────────────────────────────────────────────────────────")
		}
	}

	fmt.Println()
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println("║                     Verification                            ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println()

	// Verify all clients received all messages
	time.Sleep(200 * time.Millisecond)

	for _, tc := range []struct {
		name   string
		client *CollabClient
	}{
		{"Claudio-Bauti", bauti},
		{"Claudio-Pablo", pablo},
		{"Claudio-Chasco", chasco},
	} {
		msgs := tc.client.Messages(0)
		peers := tc.client.Peers()
		fmt.Printf("  %s: %d messages received, peers: %v\n", tc.name, len(msgs), peers)

		if len(msgs) != len(conversation) {
			t.Errorf("%s: expected %d messages, got %d", tc.name, len(conversation), len(msgs))
		}
		if len(peers) != 3 {
			t.Errorf("%s: expected 3 peers, got %d: %v", tc.name, len(peers), peers)
		}
	}

	// Verify message content integrity
	allMsgs := bauti.Messages(0)
	for i, msg := range allMsgs {
		expected := conversation[i]
		if msg.From != expected.name {
			t.Errorf("msg %d: from=%q, want %q", i, msg.From, expected.name)
		}
		if msg.Text != expected.message {
			t.Errorf("msg %d: text=%q, want %q", i, msg.Text, expected.message)
		}
		if msg.Ts == "" {
			t.Errorf("msg %d: empty timestamp", i)
		}
	}

	fmt.Println()
	fmt.Printf("  ✓ All %d messages delivered to all 3 sessions\n", len(conversation))
	fmt.Println("  ✓ Message ordering preserved")
	fmt.Println("  ✓ Peer lists correct (each sees all 3 including self)")
	fmt.Println("  ✓ Timestamps present on all messages")
	fmt.Println()
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
}
