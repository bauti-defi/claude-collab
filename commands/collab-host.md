---
name: collab-host
description: Start the claude-collab WebSocket host server in the background
---

Start the claude-collab host server in the background using the Bash tool:

```bash
${CLAUDE_PLUGIN_ROOT}/bin/claude-collab host --port ${COLLAB_PORT:-8080}
```

Run this command in the background. After it starts, confirm the port it's listening on and remind the user to set `COLLAB_URL` and `COLLAB_NAME` env vars if they haven't already.

If the binary doesn't exist at `${CLAUDE_PLUGIN_ROOT}/bin/claude-collab`, tell the user to run `make build` in the plugin directory first.
