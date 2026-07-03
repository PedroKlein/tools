# todo

Task board with Bubbletea TUI. Stores tasks as JSON files, one per scope.
Designed to share data with AI coding agents that read/write the same format.

## Usage

```bash
todo                          # Launch full TUI
todo add "fix the auth bug"   # TUI form with AI-parsed pre-fill
todo add -q "quick task"      # Skip TUI, save directly (for scripts/agents)
todo list                     # List tasks for current repo
todo list -a                  # List all tasks across scopes
todo done <id>                # Mark task done
todo update <id> [fields]     # Update task fields
todo delete <id>              # Delete a task
```

## Flags

| Flag | Description |
|------|-------------|
| `-q, --quick` | Skip TUI form (direct save, for scripts/agents) |
| `-g, --global` | Target personal scope |
| `-w, --work` | Target work scope |
| `-j, --json` | JSON output |

## TUI

Full-screen Bubbletea interface:

- **Tab bar**: switch between scopes with `h`/`l`
- **Task list**: vim navigation (`j`/`k`), priority colors, urgency sorting
- **Add/edit form**: type, priority, scope selector, due date, description
- **Filter**: cycle status filter with `f` (active / all / done / blocked)
- **Actions**: `d` toggle done, `x` delete, `n` add note, `e` edit, `a` add new

## Storage

Tasks stored in `~/.config/todo/` as JSON files, one per scope:

| File | Scope |
|------|-------|
| `global.json` | Personal tasks (not tied to a repo) |
| `work.json` | Generic work tasks |
| `reviews.json` | PR review tasks |
| `<owner__repo>.json` | Repo-specific tasks (auto-detected from CWD) |

The `__` (double underscore) separator in filenames avoids ambiguity with
dashed owner/repo names. GitHub doesn't allow `__` in owner or repo names.

## AI Parsing

When adding tasks via `todo add "text"`, the input is sent to the `q` daemon
for intelligent extraction of type, priority, and due date. Falls back to
heuristic keyword matching when the daemon is unavailable.

```bash
todo add "high priority: fix auth bug by friday"
# Parsed → type: bug, priority: high, due: next friday, title: fix auth bug
```

Requires `q` daemon running. Graceful fallback when unavailable.

## Task Schema

```json
{
  "id": 1,
  "title": "Fix authentication flow",
  "type": "bug",
  "priority": "high",
  "status": "open",
  "repoId": "owner__repo",
  "description": "Optional longer description",
  "dueDate": "2025-01-15",
  "note": "Optional note",
  "createdAt": 1719900000000,
  "updatedAt": 1719900000000
}
```

### Task Types
`feature` · `bug` · `chore` · `research` · `review` · `personal`

### Priorities
`low` · `medium` · `high`

### Statuses
`open` · `blocked` · `done`

## Integration with Agents

The JSON format is designed for AI coding agents to read and write directly.
Any tool that writes to `~/.config/todo/<scope>.json` with the schema above
will show up in the TUI. The `--quick --json` flags exist specifically for
non-interactive agent usage:

```bash
# Agent adds a task
todo add -q -w "investigate flaky test in CI"

# Agent lists tasks as JSON
todo list -j

# Agent marks done
todo done 3
```
