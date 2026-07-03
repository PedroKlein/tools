package main

import (
	"os/exec"
	"strings"
	"testing"
)

// initBareRepo creates a temporary bare git repository and returns its path.
func initBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	out, err := exec.Command("git", "init", "--bare", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}

	return dir
}

// makeRef creates a real ref in a bare repo by hashing a dummy blob object.
// This is necessary for rev-parse --verify checks to succeed.
func makeRef(t *testing.T, bareDir, ref string) {
	t.Helper()

	cmd := exec.Command("git", "hash-object", "-w", "--stdin")
	cmd.Dir = bareDir
	cmd.Stdin = strings.NewReader("dummy content for " + ref)

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hash-object for %s: %v", ref, err)
	}

	sha := strings.TrimSpace(string(out))
	upd := exec.Command("git", "update-ref", ref, sha)

	upd.Dir = bareDir
	if out, err := upd.CombinedOutput(); err != nil {
		t.Fatalf("update-ref %s %s: %v\n%s", ref, sha, err, out)
	}
}

// setSymRef creates a symbolic ref in a bare repo.
func setSymRef(t *testing.T, bareDir, ref, target string) {
	t.Helper()

	cmd := exec.Command("git", "symbolic-ref", ref, target)

	cmd.Dir = bareDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("symbolic-ref %s -> %s: %v\n%s", ref, target, err, out)
	}
}

func TestDetectDefaultBranch(t *testing.T) {
	t.Run("origin/HEAD present overrides local HEAD", func(t *testing.T) {
		dir := initBareRepo(t)
		// Pollute local HEAD with a feature branch (as happens after worktree add)
		setSymRef(t, dir, "HEAD", "refs/heads/feature-x")
		// Remote declares develop as its default
		setSymRef(t, dir, "refs/remotes/origin/HEAD", "refs/remotes/origin/develop")

		got := detectDefaultBranch(dir)
		if got != "develop" {
			t.Errorf("detectDefaultBranch() = %q, want %q", got, "develop")
		}
	})

	t.Run("origin/HEAD missing, origin/main exists", func(t *testing.T) {
		dir := initBareRepo(t)
		// No origin/HEAD symbolic ref — but origin/main is a real ref
		makeRef(t, dir, "refs/remotes/origin/main")

		got := detectDefaultBranch(dir)
		if got != "main" {
			t.Errorf("detectDefaultBranch() = %q, want %q", got, "main")
		}
	})

	t.Run("origin/HEAD missing, only origin/master exists", func(t *testing.T) {
		dir := initBareRepo(t)
		// Only origin/master is present — no origin/main, no origin/HEAD
		makeRef(t, dir, "refs/remotes/origin/master")

		got := detectDefaultBranch(dir)
		if got != "master" {
			t.Errorf("detectDefaultBranch() = %q, want %q", got, "master")
		}
	})

	t.Run("nothing exists, fallback to main", func(t *testing.T) {
		dir := initBareRepo(t)
		// Completely empty bare repo — no remote refs at all

		got := detectDefaultBranch(dir)
		if got != "main" {
			t.Errorf("detectDefaultBranch() = %q, want %q", got, "main")
		}
	})
}
