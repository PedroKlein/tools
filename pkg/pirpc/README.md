# pirpc

Thin client for the `q` daemon Unix socket. Provides prompt/response
communication with the Pi LLM subprocess.

**Internal package.** No stability guarantees for external consumers.

## API

```go
import "github.com/PedroKlein/tools/pkg/pirpc"

// Single prompt (conversational, maintains session)
response, err := pirpc.PromptOnce("what is a goroutine")

// Fresh prompt (resets session before, isolated context)
response, err := pirpc.PromptFresh("parse: fix auth bug by friday")

// Reset session (cleanup after one-shot usage)
err := pirpc.ResetSession()

// Check if daemon is reachable
if pirpc.IsAvailable() { ... }

// Socket/PID paths
pirpc.SocketPath() // ~/.cache/q/q.sock (or $XDG_RUNTIME_DIR/q.sock)
pirpc.PIDPath()    // ~/.cache/q/q.pid
```

## Protocol

Newline-delimited JSON (JSONL) over Unix socket.

**Client → Daemon:**
```json
{"type": "prompt", "message": "hello"}
{"type": "new_session"}
```

**Daemon → Client:**
```json
{"type": "message_update", "assistantMessageEvent": {"type": "text_delta", "delta": "Hi"}}
{"type": "agent_end"}
{"type": "response", "success": true}
{"type": "error", "error": "something went wrong"}
```
