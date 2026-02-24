---
name: collaborate
description: Coordinate with other Claude Code sessions over LAN using the claude-collab plugin. Use when multiple Claude sessions need to share status, divide work, or avoid conflicts on shared code.
---

# Collaboration Protocol

Coordinate with other Claude Code sessions via the claude-collab WebSocket channel.

## Available Tools

| Tool | Purpose |
|------|---------|
| `send_message` | Broadcast a message to all connected sessions |
| `read_messages` | Read recent messages from the channel (default: last 20) |
| `list_peers` | See who is currently connected |

## When to Use

- **Starting work**: Announce what you're about to work on
- **Before modifying shared files**: Check if anyone else is working on them
- **Completing work**: Summarize what you changed
- **Blocking issues**: Ask for help or flag conflicts
- **Coordinating merges**: Announce when you're about to merge

## Communication Protocol

1. **On session start**: `send_message` with `STATUS: Starting work on <description>`
2. **Before shared file edits**: `read_messages` to check for conflicts, then announce
3. **On completion**: `send_message` with `DONE: <summary of changes>`
4. **When blocked**: `send_message` with `QUESTION: <what you need>`
5. **Heads up**: `send_message` with `HEADS-UP: <warning about side effects>`

## Message Prefixes

| Prefix | Meaning |
|--------|---------|
| `STATUS:` | Current work status update |
| `DONE:` | Completed a unit of work |
| `QUESTION:` | Need input from another session |
| `HEADS-UP:` | Warning about upcoming changes or side effects |
| `CONFLICT:` | Detected a file conflict with another session |

## Examples

```
send_message("STATUS: Starting work on the lending strategy refactor in DAMM-bot/src/strategies/lending/")
```

```
send_message("HEADS-UP: About to modify shared config in DAMM-stable-API/src/config/chains.ts — hold off on changes there")
```

```
send_message("DONE: Finished lending strategy refactor. Changed 3 files in strategies/lending/. Ready for review.")
```

```
send_message("QUESTION: Anyone working on the Arbitrum RPC config? I need to update the fallback URLs.")
```

## Best Practices

- Keep messages concise and actionable
- Always use a prefix so others can scan quickly
- Check `read_messages` before starting work on shared areas
- Use `list_peers` to know who's active before asking questions
- Don't flood the channel — one status update per logical unit of work
