package reload

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestHookTmuxNoOpWhenNoConf verifies that a theme dir without
// tmux.conf under derived/ or the root is a silent no-op.
func TestHookTmuxNoOpWhenNoConf(t *testing.T) {
	tmp := t.TempDir()
	themeDir := filepath.Join(tmp, "theme")
	os.MkdirAll(filepath.Join(themeDir, "derived"), 0o755)

	if err := hookTmux(context.Background(), themeDir); err != nil {
		t.Errorf("no-conf hookTmux should be no-op, got err: %v", err)
	}
}

// TestHookTmuxReturnsNilWithoutServer verifies the exit-0 contract:
// even with a tmux.conf present, hookTmux does not fail when no tmux
// server is running (list-clients exits non-zero). Manual smoke test
// covers the "tmux is running" path — CI can't spin up a tmux server
// reliably.
func TestHookTmuxReturnsNilWithoutServer(t *testing.T) {
	tmp := t.TempDir()
	themeDir := filepath.Join(tmp, "theme")
	os.MkdirAll(filepath.Join(themeDir, "derived"), 0o755)
	os.WriteFile(filepath.Join(themeDir, "derived", "tmux.conf"), []byte("# empty\n"), 0o644)

	// Force tmux operations to run against a socket that doesn't exist
	// by setting TMUX_TMPDIR to a dir with no server. list-clients on a
	// missing server returns non-zero; hookTmux must still return nil.
	origTmuxDir := os.Getenv("TMUX_TMPDIR")
	os.Setenv("TMUX_TMPDIR", tmp) // no server here
	t.Cleanup(func() { os.Setenv("TMUX_TMPDIR", origTmuxDir) })

	// Also unset TMUX (which points at an active session for the caller).
	origTmux := os.Getenv("TMUX")
	os.Unsetenv("TMUX")
	t.Cleanup(func() { os.Setenv("TMUX", origTmux) })

	if err := hookTmux(context.Background(), themeDir); err != nil {
		t.Errorf("no-server hookTmux should return nil, got: %v", err)
	}
}
