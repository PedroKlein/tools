package main

import (
	"os"
	"path/filepath"
	"testing"
)

//nolint:gocognit,gocyclo // table-driven test with many sub-cases covering all isBareRepo variations
func TestIsBareRepo(t *testing.T) {
	t.Run("bare repo with bare = true", func(t *testing.T) {
		dir := t.TempDir()

		gitDir := filepath.Join(dir, ".git")
		if err := os.MkdirAll(gitDir, 0o750); err != nil {
			t.Fatal(err)
		}

		cfg := "[core]\n\trepositoryformatversion = 0\n\tbare = true\n"
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}

		if !isBareRepo(dir) {
			t.Error("expected isBareRepo to return true for bare repo")
		}
	})

	t.Run("normal clone with bare = false", func(t *testing.T) {
		dir := t.TempDir()

		gitDir := filepath.Join(dir, ".git")
		if err := os.MkdirAll(gitDir, 0o750); err != nil {
			t.Fatal(err)
		}

		cfg := "[core]\n\trepositoryformatversion = 0\n\tbare = false\n"
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}

		if isBareRepo(dir) {
			t.Error("expected isBareRepo to return false for non-bare repo")
		}
	})

	t.Run(".git is a file (worktree pointer)", func(t *testing.T) {
		dir := t.TempDir()

		gitFile := filepath.Join(dir, ".git")
		if err := os.WriteFile(gitFile, []byte("gitdir: /path/to/bare/worktrees/feat\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		if isBareRepo(dir) {
			t.Error("expected isBareRepo to return false when .git is a file")
		}
	})

	t.Run("no .git at all", func(t *testing.T) {
		dir := t.TempDir()
		if isBareRepo(dir) {
			t.Error("expected isBareRepo to return false for empty directory")
		}
	})

	t.Run("bare = true outside [core] section", func(t *testing.T) {
		dir := t.TempDir()

		gitDir := filepath.Join(dir, ".git")
		if err := os.MkdirAll(gitDir, 0o750); err != nil {
			t.Fatal(err)
		}

		cfg := "[other]\n\tbare = true\n[core]\n\tbare = false\n"
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}

		if isBareRepo(dir) {
			t.Error("expected isBareRepo to return false when bare=true is not under [core]")
		}
	})

	t.Run("bare = true with extra whitespace", func(t *testing.T) {
		dir := t.TempDir()

		gitDir := filepath.Join(dir, ".git")
		if err := os.MkdirAll(gitDir, 0o750); err != nil {
			t.Fatal(err)
		}

		cfg := "[core]\n  bare = true\n"
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}

		if !isBareRepo(dir) {
			t.Error("expected isBareRepo to return true with leading whitespace on bare = true")
		}
	})

	t.Run("bare=true without spaces around equals", func(t *testing.T) {
		dir := t.TempDir()

		gitDir := filepath.Join(dir, ".git")
		if err := os.MkdirAll(gitDir, 0o750); err != nil {
			t.Fatal(err)
		}

		cfg := "[core]\n\tbare=true\n"
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}

		if !isBareRepo(dir) {
			t.Error("expected isBareRepo to return true for bare=true without spaces")
		}
	})
}

func TestReposRoot(t *testing.T) {
	tests := []struct {
		name   string
		envVal string
		want   string
	}{
		{
			name:   "env var set",
			envVal: "/tmp/test-repos",
			want:   "/tmp/test-repos",
		},
		{
			name:   "env var with tilde",
			envVal: "~/custom-repos",
			want:   homeDir() + "/custom-repos",
		},
		{
			name:   "env var empty — default",
			envVal: "",
			want:   homeDir() + "/Dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				t.Setenv("REPOS_ROOT", tt.envVal)
			} else {
				t.Setenv("REPOS_ROOT", "")
			}

			got := ReposRoot()
			if got != tt.want {
				t.Errorf("ReposRoot() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsGitRepo(t *testing.T) {
	t.Run("bare repo", func(t *testing.T) {
		dir := t.TempDir()

		gitDir := filepath.Join(dir, ".git")
		if err := os.MkdirAll(gitDir, 0o750); err != nil {
			t.Fatal(err)
		}

		cfg := "[core]\n\tbare = true\n"
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}

		if !isGitRepo(dir) {
			t.Error("expected isGitRepo to return true for bare repo")
		}
	})

	t.Run("non-bare repo", func(t *testing.T) {
		dir := t.TempDir()

		gitDir := filepath.Join(dir, ".git")
		if err := os.MkdirAll(gitDir, 0o750); err != nil {
			t.Fatal(err)
		}

		cfg := "[core]\n\tbare = false\n"
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}

		if !isGitRepo(dir) {
			t.Error("expected isGitRepo to return true for non-bare repo")
		}
	})

	t.Run(".git is a file", func(t *testing.T) {
		dir := t.TempDir()

		gitFile := filepath.Join(dir, ".git")
		if err := os.WriteFile(gitFile, []byte("gitdir: /path/to/bare/worktrees/feat\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		if isGitRepo(dir) {
			t.Error("expected isGitRepo to return false when .git is a file")
		}
	})

	t.Run("no .git", func(t *testing.T) {
		dir := t.TempDir()
		if isGitRepo(dir) {
			t.Error("expected isGitRepo to return false for empty directory")
		}
	})

	t.Run(".git dir without config", func(t *testing.T) {
		dir := t.TempDir()

		gitDir := filepath.Join(dir, ".git")
		if err := os.MkdirAll(gitDir, 0o750); err != nil {
			t.Fatal(err)
		}

		if isGitRepo(dir) {
			t.Error("expected isGitRepo to return false when .git has no config")
		}
	})
}
