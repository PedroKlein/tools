# gitrepo

Git remote URL parsing, slug generation, and CWD repo detection.

**Internal package.** No stability guarantees for external consumers.

## API

```go
import "github.com/PedroKlein/tools/pkg/gitrepo"

// Parse any git remote URL into components
info, err := gitrepo.ParseRemote("git@github.com:Owner/repo.git")
// info.Host = "github.com", info.Owner = "Owner", info.Repo = "repo"

// Generate a slug for file naming (double-underscore separator)
slug := gitrepo.SlugFromRemote("git@github.com:Owner/repo.git")
// → "owner__repo" (lowercased)

// Detect current repo from working directory (runs git commands)
slug, err := gitrepo.DetectFromCWD()
// → "owner__repo"
```

## Slug Format

`owner__repo` (double underscore, lowercased). GitHub doesn't allow `__` in
owner or repo names, making this separator unambiguous for splitting.

## Supported URL Formats

- `git@host:owner/repo.git` (SSH)
- `https://host/owner/repo.git` (HTTPS)
- `ssh://git@host/owner/repo` (SSH with scheme)
- Works with any host (GitHub, GitLab, Bitbucket, enterprise instances)
