package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// newMCPServer creates an MCP server backed by any Collaborator.
func newMCPServer(client Collaborator) *server.MCPServer {
	s := server.NewMCPServer(
		"claude-collab",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	s.AddTool(sendMessageTool(), sendMessageHandler(client))
	s.AddTool(readMessagesTool(), readMessagesHandler(client))
	s.AddTool(listPeersTool(), listPeersHandler(client))

	return s
}

// RunMCP connects to the collab server and starts the MCP stdio bridge.
func RunMCP(wsURL, name string) error {
	client, err := Connect(wsURL, name)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	s := newMCPServer(client)

	fmt.Fprintf(os.Stderr, "claude-collab: connected as %q to %s\n", name, wsURL)
	return server.ServeStdio(s)
}

// --- send_message ---

func sendMessageTool() mcp.Tool {
	return mcp.NewTool("send_message",
		mcp.WithDescription("Send a message to all connected Claude Code sessions."),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("The message text to broadcast to all peers."),
		),
	)
}

func sendMessageHandler(client Collaborator) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text, err := req.RequireString("text")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := client.Send(text); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("send failed: %v", err)), nil
		}

		n := client.PeerCount()
		return mcp.NewToolResultText(fmt.Sprintf("Message sent to %d peer(s).", n)), nil
	}
}

// --- read_messages ---

func readMessagesTool() mcp.Tool {
	return mcp.NewTool("read_messages",
		mcp.WithDescription("Read recent messages from the collaboration channel."),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of messages to return. Defaults to 20."),
		),
	)
}

func readMessagesHandler(client Collaborator) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := req.GetInt("limit", 20)

		msgs := client.Messages(limit)
		if len(msgs) == 0 {
			return mcp.NewToolResultText("No messages yet."), nil
		}

		data, err := json.Marshal(msgs)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- list_peers ---

func listPeersTool() mcp.Tool {
	return mcp.NewTool("list_peers",
		mcp.WithDescription("List all currently connected Claude Code sessions."),
	)
}

func listPeersHandler(client Collaborator) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		peers := client.Peers()
		if len(peers) == 0 {
			return mcp.NewToolResultText("No other peers connected."), nil
		}

		return mcp.NewToolResultText(
			fmt.Sprintf("Connected peers (%d): %s", len(peers), strings.Join(peers, ", ")),
		), nil
	}
}
