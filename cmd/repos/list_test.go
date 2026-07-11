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
	bareRepos := []string{
		"github.com/PedroKlein/guri",
		"github.com/PedroKlein/pi-baml",
		"git.example.corp/start/auth-service",
		"git.corp.internal/acme-platform/platform-docs",
	}

	for _, r := range bareRepos {
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

	// Non-bare repos (have .git/ with core.bare = false) — should also be detected
	flatRepos := []string{
		"github.com/someone/normal-clone",
		"github.tools.sap/org/infra",
	}

	for _, r := range flatRepos {
		gitDir := filepath.Join(root, r, ".git")

		err := os.MkdirAll(gitDir, 0o750)
		if err != nil {
			t.Fatal(err)
		}

		cfg := "[core]\n\tbare = false\n"
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	allRepos := append(bareRepos, flatRepos...)

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

	t.Run("lists all repos including non-bare", func(t *testing.T) {
		repos, err := ListRepos(root, "")
		if err != nil {
			t.Fatal(err)
		}

		if len(repos) != len(allRepos) {
			t.Errorf("got %d repos, want %d", len(repos), len(allRepos))

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

func TestFilterReposTiered(t *testing.T) {
	repos := []string{
		"github.com/PedroKlein/tools",
		"github.com/someone/dev-tools",
		"github.tools.sap/org/infra",
		"github.tools.sap/org/platform",
		"github.com/acme/pi-baml",
		"github.com/acme/pi-extensions",
		"github.com/acme/provision-api",
	}

	t.Run("tier 1: exact repo name", func(t *testing.T) {
		result := filterReposTiered(repos, "tools")
		if len(result) != 1 {
			t.Fatalf("got %d results, want 1: %v", len(result), result)
		}

		if result[0] != "github.com/PedroKlein/tools" {
			t.Errorf("got %q, want github.com/PedroKlein/tools", result[0])
		}
	})

	t.Run("tier 2: repo name contains", func(t *testing.T) {
		result := filterReposTiered(repos, "pi-")
		if len(result) != 2 {
			t.Fatalf("got %d results, want 2: %v", len(result), result)
		}
	})

	t.Run("tier 3: owner/repo contains", func(t *testing.T) {
		result := filterReposTiered(repos, "pedroklein")
		if len(result) != 1 {
			t.Fatalf("got %d results, want 1: %v", len(result), result)
		}
	})

	t.Run("tier 4: full path contains", func(t *testing.T) {
		result := filterReposTiered(repos, "github.tools")
		if len(result) != 2 {
			t.Fatalf("got %d results, want 2: %v", len(result), result)
		}
	})

	t.Run("exact name wins over host match", func(t *testing.T) {
		result := filterReposTiered(repos, "infra")
		// "infra" is exact name match for github.tools.sap/org/infra
		if len(result) != 1 {
			t.Fatalf("got %d results, want 1: %v", len(result), result)
		}

		if result[0] != "github.tools.sap/org/infra" {
			t.Errorf("got %q, want github.tools.sap/org/infra", result[0])
		}
	})

	t.Run("no match returns nil", func(t *testing.T) {
		result := filterReposTiered(repos, "nonexistent")
		if len(result) != 0 {
			t.Errorf("got %d results, want 0: %v", len(result), result)
		}
	})
}
