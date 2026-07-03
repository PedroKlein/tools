# AGENTS.md

## Overview

Personal Go CLI tools monorepo. Four binaries, two shared internal packages. Stdlib-heavy, single-binary installs, Unix-philosophy design.

## Repo Structure

```
cmd/
├── q/          # Persistent LLM daemon for shell access (all code in package main)
├── pia/        # Pi coding agent profile launcher (all code in package main)
├── repos/      # ~/Dev repo manager, bare+worktree layout (all code in package main)
└── todo/       # Task board with Bubbletea TUI (all code in package main, including TUI)
pkg/
├── pirpc/      # Shared: thin client for q daemon Unix socket (JSONL protocol)
└── gitrepo/    # Shared: git remote URL parsing, slug generation, CWD detection
mise.toml       # Build tasks (build, test, lint, install)
.golangci.yaml  # Linter config (adapted from production Go project)
```

## Conventions

- **Go 1.24+**, single `go.mod` at root
- **No external deps** except Charm libraries (bubbletea, lipgloss, bubbles) for TUI
- **`--json` flag** on all read commands for scripting
- **Exit codes**: 0=success, 1=error, 2=ambiguous, 3=not-found
- **Tests**: white-box (`package main`), table-driven, no testify
- **Error handling**: wrap with `%w`, no sentinel errors file, return early
- **Naming**: domain-specific names, no generic "Service" suffix
- **Data path**: `~/.config/todo/` for tasks, `~/.cache/q/q.sock` for daemon socket
- **Slug format**: `owner__repo` (double underscore) for file naming
- **Package layout**: each cmd/ is fully self-contained (package main). Only truly shared code goes in pkg/.

## Build

```bash
mise run build     # → bin/
mise run test      # go test ./...
mise run lint      # golangci-lint run
mise run install   # go install ./cmd/...
```

## Key Relationships

- `cmd/todo` writes to `~/.config/todo/*.json` — same format as pi-todo extension in dotfiles
- `cmd/pia` manages profiles defined in dotfiles (`~/.pi/agent/profiles/`)
- `cmd/q` daemon provides LLM access; `cmd/todo` connects to it via `pkg/pirpc` for AI parsing
- `cmd/repos` manages the `~/Dev` directory layout; `cmd/todo` uses `pkg/gitrepo` for CWD detection
