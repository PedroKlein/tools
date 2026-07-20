package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGitCmdQuietSurfacesStderr guards the fix for the silent-clone bug:
// gitCmdQuiet must fold captured git stderr into the returned error message
// so callers that surface the error get an actionable diagnostic instead of
// a bare "exit status N".
func TestGitCmdQuietSurfacesStderr(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Init an empty non-bare repo so `git worktree add` has a valid gitdir
	// to run against.
	if err := gitCmdQuiet(dir, "init", "--quiet", dir); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// worktree add against a nonexistent branch fails with a specific stderr
	// message; the returned error must include it.
	err := gitCmdQuiet(dir, "worktree", "add", filepath.Join(dir, "wt"), "no-such-branch-xyz")
	if err == nil {
		t.Fatal("expected worktree add to fail on unknown branch")
	}

	msg := err.Error()
	if !strings.Contains(msg, "no-such-branch-xyz") && !strings.Contains(msg, "invalid reference") {
		t.Fatalf("error message should include captured git stderr, got: %q", msg)
	}
}

// TestGitCmdQuietSilentOnSuccess ensures we still don't leak stdout to the
// parent process on success — callers depend on quiet behavior.
func TestGitCmdQuietSilentOnSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Capture stdout/stderr file descriptors around a successful git call.
	origStdout, origStderr := os.Stdout, os.Stderr

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	os.Stdout = w
	os.Stderr = w

	initErr := gitCmdQuiet(dir, "init", "--quiet", dir)

	_ = w.Close()

	os.Stdout, os.Stderr = origStdout, origStderr

	if initErr != nil {
		t.Fatalf("git init: %v", initErr)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)

	if n > 0 {
		t.Fatalf("gitCmdQuiet leaked output on success: %q", string(buf[:n]))
	}
}
