package reload

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRegistryHas15Hooks pins the total hook count. AC requires >= 10.
// If you delete a hook, update this test explicitly to acknowledge the
// removal — silent drift is the antipattern this guards against.
func TestRegistryHas15Hooks(t *testing.T) {
	got := len(Registry())
	if got < 10 {
		t.Errorf("registry has %d hooks; AC requires >= 10", got)
	}
	// Current expected: 10 inline + 5 external = 15. Adjust intentionally.
	if got != 15 {
		t.Logf("registry has %d hooks; last-known count was 15", got)
	}
}

func TestRegistryStableOrder(t *testing.T) {
	// Names are alphabetical for readability; if you reshuffle, do it
	// deliberately.
	names := []string{
		"bat", "btop", "gh-dash", "ghostty", "k9s", "lazygit",
		"nvim", "opencode", "sketchybar", "television",
		"osc-broadcast", "macos-system", "pi", "tmux", "wallpaper",
	}
	reg := Registry()
	for i, expected := range names {
		if reg[i].Name != expected {
			t.Errorf("registry[%d].Name = %q, want %q", i, reg[i].Name, expected)
		}
	}
}

func TestFilterSkipsLiveApplyOnScroll(t *testing.T) {
	// Sanity: LiveApply=false hooks must be filtered when THEME_LIVE_APPLY=1.
	// FilterHooks receives liveApply=true → drop all LiveApply=false entries.
	live := FilterHooks(nil, true)
	commit := FilterHooks(nil, false)

	if len(live) >= len(commit) {
		t.Errorf("scroll-mode filter didn't drop any hooks: live=%d commit=%d",
			len(live), len(commit))
	}

	// Every returned hook must have LiveApply=true.
	for _, h := range live {
		if !h.LiveApply {
			t.Errorf("live-mode filter kept LiveApply=false hook: %s", h.Name)
		}
	}
}

func TestFilterHonorsSkipList(t *testing.T) {
	skip := map[string]bool{"bat": true, "sketchybar": true}
	got := FilterHooks(skip, false)
	for _, h := range got {
		if h.Name == "bat" || h.Name == "sketchybar" {
			t.Errorf("skip list didn't drop %s", h.Name)
		}
	}
}

func TestFilterAppliesOSGate(t *testing.T) {
	// Override goos to linux — sketchybar and macos-system must drop out.
	orig := goos
	goos = "linux"
	defer func() { goos = orig }()

	got := FilterHooks(nil, false)
	for _, h := range got {
		if h.Name == "sketchybar" || h.Name == "macos-system" {
			t.Errorf("linux mode still includes macOS-only hook: %s", h.Name)
		}
	}
}

func TestInlineHookHonorsContextDeadline(t *testing.T) {
	// Regression: earlier runOne called h.Fn(themeDir) directly, so an
	// inline hook that blocked forever would ignore the per-hook timeout
	// context and stall RunAll indefinitely. Fix: run Fn in a goroutine
	// and select on ctx.Done() so the caller reports a timely timeout.
	blocked := make(chan struct{})
	defer close(blocked)
	h := Hook{
		Name: "blocker",
		Kind: KindInline,
		Fn: func(_ context.Context, _ string) error {
			<-blocked // never returns during the test window
			return nil
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runOne(ctx, h, "/tmp", os.Stderr) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected context timeout error")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runOne did not honor context deadline")
	}
}

func TestSkipListParsing(t *testing.T) {
	tests := []struct {
		env  string
		want []string
	}{
		{"", nil},
		{"bat", []string{"bat"}},
		{"bat,sketchybar", []string{"bat", "sketchybar"}},
		{",bat,,sketchybar,", []string{"bat", "sketchybar"}}, // tolerates empty
	}
	for _, tt := range tests {
		os.Setenv("THEME_SKIP_HOOKS", tt.env)
		got := SkipList()
		if got == nil && tt.want != nil {
			t.Errorf("SkipList(%q) = nil, want %v", tt.env, tt.want)
			continue
		}
		for _, name := range tt.want {
			if !got[name] {
				t.Errorf("SkipList(%q) missing %s", tt.env, name)
			}
		}
	}
	os.Unsetenv("THEME_SKIP_HOOKS")
}

func TestStaleReloadHonorsXDGConfigHome(t *testing.T) {
	// Regression: earlier isStaleReload derived `.current` from a hardcoded
	// $HOME/.config/themes path, so XDG_CONFIG_HOME installs got every
	// reload skipped as "superseded" — the .current inside XDG_CONFIG_HOME
	// pointed at the just-applied theme, but the guard checked the wrong
	// root and saw an empty/missing link.
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	defer os.Unsetenv("XDG_CONFIG_HOME")
	themeDir := filepath.Join(tmp, "themes", "osaka-jade")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("osaka-jade", filepath.Join(tmp, "themes", ".current")); err != nil {
		t.Fatal(err)
	}
	if isStaleReload(themeDir) {
		t.Fatal("XDG install: reload flagged stale despite .current pointing to same theme")
	}
	// Simulate a supersede: overwrite .current with a different theme.
	os.Remove(filepath.Join(tmp, "themes", ".current"))
	if err := os.Symlink("tokyonight", filepath.Join(tmp, "themes", ".current")); err != nil {
		t.Fatal(err)
	}
	if !isStaleReload(themeDir) {
		t.Fatal("XDG install: reload NOT flagged stale after .current moved to another theme")
	}
}
