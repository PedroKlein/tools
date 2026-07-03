# repos

Git repo manager that organizes repositories in a canonical
`host/owner/repo` layout using bare clones with worktrees.

## Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `REPOS_ROOT` env var | `~/Dev` | Root directory for all managed repos |
| `~/.config/repos/hooks.json` | (none) | Lifecycle hooks definition |

No config file required. Works with defaults out of the box.

## Usage

```bash
repos clone <url>             # Bare clone + default branch worktree
repos list [query]            # List repos (case-insensitive substring match)
repos info <query>            # Show details (remote, branches, worktrees, dirty status)
repos path <query>            # Print absolute path (for scripting)
repos audit [query]           # Check for dirty worktrees / unpushed branches
repos sync [query]            # Fetch + fast-forward default branch
repos tidy [query]            # Fix stale HEAD refs, prune stale worktrees
repos wt [branch]             # Context-aware worktree create/list/remove
repos open [query]            # Open repo in tmux + nvim session
repos browse [query]          # Open repo URL in browser
repos exec <query> -- <cmd>   # Run command across matching repos
repos export [-o file]        # Export all repos to JSON manifest
repos import <file>           # Clone repos from JSON manifest
repos migrate [path]          # Convert existing repo to canonical layout
repos hook <list|add|rm>      # Manage lifecycle hooks
repos rm <query>              # Remove repo (with confirmation)
```

All read commands support `--json` for structured output.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Error |
| 2 | Ambiguous query (multiple matches) |
| 3 | Not found |

## Directory Layout

After cloning, repos are stored as bare git repositories with worktrees:

```
$REPOS_ROOT/
├── github.com/
│   ├── owner/
│   │   └── repo/            # bare git repo (HEAD, objects, refs)
│   │       ├── main/        # default branch worktree
│   │       └── feature-x/   # additional worktree
│   └── another-org/
│       └── project/
└── gitlab.com/
    └── team/
        └── service/
```

This layout:
- Avoids remote URL collisions (same repo name, different hosts/owners)
- Supports multiple worktrees per repo without separate clones
- Works with any git host (GitHub, GitLab, Bitbucket, self-hosted)

## Worktrees

`repos wt` is context-aware (detects current repo from CWD):

```bash
repos wt                    # List worktrees for current repo
repos wt feature-auth       # Create worktree (prints path to stdout)
cd $(repos wt feature-auth) # Create and cd in one step
repos wt -d feature-auth    # Remove worktree
```

Use `-r <query>` to target a different repo than the current CWD.

## Hooks

Lifecycle hooks fire after operations. Defined in `~/.config/repos/hooks.json`:

```json
{
  "post-clone": ["~/.config/repos/hooks/setup.sh"],
  "post-sync": [],
  "post-tidy": []
}
```

Hook scripts receive context via environment variables:

| Variable | Example |
|----------|---------|
| `REPO_HOST` | `github.com` |
| `REPO_OWNER` | `PedroKlein` |
| `REPO_NAME` | `tools` |
| `REPO_PATH` | `/home/user/Dev/github.com/PedroKlein/tools` |

## Shell Integration

```bash
# Fuzzy cd into any managed repo
rcd() { cd "$(repos list --json | jq -r '.[].path' | fzf)" }

# Direct path access for scripting
cd $(repos path myproject)

# Audit all repos before EOD
repos audit
```
