package reload

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestHookTmuxUsesStatusOnlyRefresh(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(tmp, "tmux.log")
	fakeTmux := filepath.Join(bin, "tmux")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$TMUX_TEST_LOG"
case "$*" in
  *list-clients*) printf 'client-one\n' ;;
esac
exit 0
`
	if err := os.WriteFile(fakeTmux, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX_TEST_LOG", logPath)
	t.Setenv("TMUX", "/tmp/themes-test-tmux,123,0")

	themeDir := filepath.Join(tmp, "theme")
	if err := os.MkdirAll(filepath.Join(themeDir, "derived"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "derived", "tmux.conf"), []byte("# empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := hookTmux(context.Background(), themeDir); err != nil {
		t.Fatalf("hookTmux returned error: %v", err)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logBytes)
	if !strings.Contains(log, "-S /tmp/themes-test-tmux refresh-client -S -t client-one") {
		t.Fatalf("hookTmux did not status-refresh client; log:\n%s", log)
	}
	if strings.Contains(log, "refresh-client -t client-one") {
		t.Fatalf("hookTmux used full client refresh; log:\n%s", log)
	}
}

func TestHookTmuxPreviewOutsideTmuxNoOps(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(tmp, "tmux.log")
	fakeTmux := filepath.Join(bin, "tmux")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$TMUX_TEST_LOG"
exit 0
`
	if err := os.WriteFile(fakeTmux, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX_TEST_LOG", logPath)
	t.Setenv("TMUX", "")

	themeDir := filepath.Join(tmp, "theme")
	if err := os.MkdirAll(filepath.Join(themeDir, "derived"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "derived", "tmux.conf"), []byte("# empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(context.Background(), liveApplyContextKey{}, true)
	if err := hookTmux(ctx, themeDir); err != nil {
		t.Fatalf("hookTmux returned error: %v", err)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("preview outside tmux ran tmux commands; stat err=%v", err)
	}
}
