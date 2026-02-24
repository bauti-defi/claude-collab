package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		usage()
	}

	switch args[0] {
	case "host":
		port := 8080
		for i := 1; i < len(args)-1; i++ {
			if args[i] == "--port" {
				p, err := strconv.Atoi(args[i+1])
				if err != nil {
					fatal("invalid port: %s", args[i+1])
				}
				port = p
			}
		}
		if err := RunServer(port); err != nil {
			fatal("server error: %v", err)
		}

	case "join":
		var url, name string
		for i := 1; i < len(args)-1; i++ {
			switch args[i] {
			case "--url":
				url = args[i+1]
			case "--name":
				name = args[i+1]
			}
		}
		if url == "" || name == "" {
			fatal("join requires --url and --name")
		}
		if err := RunMCP(url, name); err != nil {
			fatal("mcp error: %v", err)
		}

	default:
		usage()
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `claude-collab — LAN collaboration for Claude Code sessions

Usage:
  claude-collab host [--port PORT]           Start WebSocket server (default: 8080)
  claude-collab join --url URL --name NAME   Start MCP stdio client
`)
	os.Exit(1)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "claude-collab: "+format+"\n", args...)
	os.Exit(1)
}
