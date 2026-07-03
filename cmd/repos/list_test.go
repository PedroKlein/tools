package main

import (
	"os"
	"path/filepath"
	"testing"
)

//nolint:gocognit,gocyclo // table-driven test with many sub-cases for repo discovery logic
func TestListRepos(t *testing.T) {
	// Create temp directory structure
	root := t.TempDir()

	// Valid repos (have .git/ with core.bare = true at depth 3)
	validRepos := []string{
		"github.com/PedroKlein/guri",
		"github.com/PedroKlein/pi-baml",
		"git.example.corp/start/auth-service",
		"git.corp.internal/acme-platform/platform-docs",
	}

	for _, r := range validRepos {
		gitDir := filepath.Join(root, r, ".git")

		err := os.MkdirAll(gitDir, 0o750)
		if err != nil {
			t.Fatal(err)
		}

		cfg := "[core]\n\tbare = true\n"
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Non-repo dirs (should be ignored)
	nonRepos := []string{
		"experiments/stuff",
		"github.com/PedroKlein/not-a-repo/subdir",
		"_local/scratch",
	}
	for _, r := range nonRepos {
		err := os.MkdirAll(filepath.Join(root, r), 0o750)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Non-bare .git at depth 3 — should NOT be detected
	normalClone := "github.com/someone/normal-clone"
	{
		gitDir := filepath.Join(root, normalClone, ".git")
		if err := os.MkdirAll(gitDir, 0o750); err != nil {
			t.Fatal(err)
		}

		cfg := "[core]\n\tbare = false\n"
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("lists all repos", func(t *testing.T) {
		repos, err := ListRepos(root, "")
		if err != nil {
			t.Fatal(err)
		}

		if len(repos) != len(validRepos) {
			t.Errorf("got %d repos, want %d", len(repos), len(validRepos))

			for _, r := range repos {
				t.Logf("  found: %s", r)
			}
		}
	})

	t.Run("filter by query", func(t *testing.T) {
		repos, err := ListRepos(root, "auth")
		if err != nil {
			t.Fatal(err)
		}

		if len(repos) != 1 {
			t.Errorf("got %d repos, want 1", len(repos))
		}

		if len(repos) > 0 && repos[0] != "git.example.corp/start/auth-service" {
			t.Errorf("got %q, want git.example.corp/start/auth-service", repos[0])
		}
	})

	t.Run("filter by owner", func(t *testing.T) {
		repos, err := ListRepos(root, "PedroKlein")
		if err != nil {
			t.Fatal(err)
		}

		if len(repos) != 2 {
			t.Errorf("got %d repos, want 2", len(repos))
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		repos, err := ListRepos(root, "nonexistent")
		if err != nil {
			t.Fatal(err)
		}

		if len(repos) != 0 {
			t.Errorf("got %d repos, want 0", len(repos))
		}
	})

	t.Run("case insensitive filter", func(t *testing.T) {
		repos, err := ListRepos(root, "pedroklein")
		if err != nil {
			t.Fatal(err)
		}

		if len(repos) != 2 {
			t.Errorf("got %d repos, want 2", len(repos))
		}
	})
}
