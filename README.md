# tools

Personal CLI tools. Built with Go, stdlib-heavy, single-binary installs.

These are opinionated tools for my workflow. They solve problems the existing
alternatives didn't solve well enough for me. You're welcome to use them, fork
them, or steal ideas.

## Install

```bash
go install github.com/PedroKlein/tools/cmd/...@latest
```

Or clone and build locally:

```bash
git clone https://github.com/PedroKlein/tools.git
cd tools
mise run install  # or: go install ./cmd/...
```

Requires Go 1.24+.

## Tools

| Command | Description |
|---------|-------------|
| [`q`](cmd/q/) | Persistent LLM daemon for instant shell access via Unix socket |
| [`pia`](cmd/pia/) | Pi coding agent profile launcher and manager |
| [`repos`](cmd/repos/) | Git repo manager with canonical `host/owner/repo` layout |
| [`todo`](cmd/todo/) | Task board with Bubbletea TUI, JSON file storage |

## Shared Packages

Internal packages under `pkg/`. Not intended for external import (no stability guarantees).

| Package | Purpose |
|---------|---------|
| [`pkg/pirpc`](pkg/pirpc/) | Thin client for the `q` daemon Unix socket (JSONL protocol) |
| [`pkg/gitrepo`](pkg/gitrepo/) | Git remote URL parsing, slug generation, CWD repo detection |

## Design Principles

- **Stdlib-only where possible.** External deps only for TUI (bubbletea/lipgloss). No frameworks.
- **Single binary per tool.** No runtime dependencies, no config files required to start.
- **Unix philosophy.** Small tools that compose. JSON output (`--json`) for scripting. Exit codes matter.
- **Fast startup.** Sub-100ms. These run in shell aliases and tmux hooks.

## Development

Uses [mise](https://mise.jdx.dev/) for task running:

```bash
mise run build     # compile all binaries to bin/
mise run test      # go test ./...
mise run lint      # golangci-lint run
mise run install   # go install all commands
```

## License

MIT
