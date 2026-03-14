package main

import (
	"net/http"

	"github.com/mark3labs/mcp-go/server"
)

// newMCPHTTPHandler creates a Streamable HTTP handler for the remote MCP endpoint.
// This allows claude.ai (or any MCP-compatible client) to connect over HTTP.
func newMCPHTTPHandler(client Collaborator) http.Handler {
	s := newMCPServer(client)
	httpServer := server.NewStreamableHTTPServer(s, server.WithStateLess(true))
	return withCORS(httpServer)
}

// withCORS wraps an http.Handler with permissive CORS headers.
// Required for claude.ai to reach the endpoint via tunnel (different origin).
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Mcp-Session-Id")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
