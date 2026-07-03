# q

Persistent LLM daemon for instant shell access. Single daemon per user, shared
across all terminal sessions via Unix socket.

## Requirements

- [Pi coding agent](https://github.com/mariozechner/pi-coding-agent) installed and in PATH
- A configured model (the daemon spawns `pi` as a subprocess)

## Usage

```bash
q <prompt>                  # Ask a question (auto-starts daemon)
q -p <profile> <prompt>     # Use a specific profile
q --run <prompt>            # Get a command and execute it (with confirmation)
q --new <prompt>            # Reset session before prompting
q -c <prompt>              # Copy response to clipboard
echo "log" | q "explain"   # Pipe stdin as context
q start                     # Pre-warm daemon
q stop                      # Kill daemon
q reset                     # Clear session (keep daemon running)
q status                    # Show daemon status (pid, socket)
q profiles                  # List available profiles
```

## Configuration

Optional config file at `~/.config/q/config.json`:

```json
{
  "model": "your-model-id",
  "systemPrompt": "You are a concise shell assistant. Output raw text, no code fences."
}
```

Without a config file, q uses sensible defaults (concise shell assistant persona).

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `model` | string | Model identifier passed to Pi |
| `systemPrompt` | string | Base instructions for the LLM (set once at daemon start) |

## Profiles

Profiles shape responses by prepending instructions per-request. They don't
change the model or system prompt, just add context to a single query.

**Built-in profiles** (always available):

| Profile | Behavior |
|---------|----------|
| `cmd` | Output a single shell command, no explanation |
| `explain` | Detailed explanation with examples |
| `code` | Code-only output, no prose |
| `think` | Step-by-step reasoning before answering |

**Custom profiles**: create `~/.config/q/profiles/<name>.txt` with plain-text
instructions. Custom profiles override built-ins if they share a name.

## Architecture

- Single daemon process, Unix socket at `~/.cache/q/q.sock` (or `$XDG_RUNTIME_DIR/q.sock`)
- Spawns `pi --no-session` subprocess for LLM inference
- Idle timeout resets conversation context (daemon stays alive indefinitely)
- Auto-starts on first invocation; stays resident until `q stop`
- Socket-based protocol: JSONL messages (see `pkg/pirpc/` for the wire format)

## Environment

| Variable | Effect |
|----------|--------|
| `Q_SOCKET` | Override socket path |
| `XDG_RUNTIME_DIR` | Used for socket directory on Linux |

## Shell Integration

```bash
alias qn='q --new'     # force fresh context
alias qr='q --run'     # run mode
alias qt='q -p think'  # thinking mode
```

Add `q start` to `.zshrc` for instant first-query response (daemon pre-warm).
