package state

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRootHonorsXDGStateHome verifies AC-2: paths resolve to
// $XDG_STATE_HOME/themes when set, and fall back to ~/.local/state/themes
// when unset.
func TestRootHonorsXDGStateHome(t *testing.T) {
	orig := os.Getenv("XDG_STATE_HOME")
	t.Cleanup(func() { os.Setenv("XDG_STATE_HOME", orig) })

	os.Setenv("XDG_STATE_HOME", "/tmp/xdg-state-test")
	if got := Root(); got != "/tmp/xdg-state-test/themes" {
		t.Errorf("XDG-set Root = %q, want /tmp/xdg-state-test/themes", got)
	}

	os.Unsetenv("XDG_STATE_HOME")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no HOME available: %v", err)
	}
	want := filepath.Join(home, ".local", "state", "themes")
	if got := Root(); got != want {
		t.Errorf("fallback Root = %q, want %q", got, want)
	}
}

// TestSaveAndLoadRoundTrip verifies AC-1: Save + Load round-trip
// preserves fields; missing file loads to zero-valued State (no error).
func TestSaveAndLoadRoundTrip(t *testing.T) {
	// Force XDG_STATE_HOME into a temp dir so we don't touch the real one.
	orig := os.Getenv("XDG_STATE_HOME")
	tmp := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmp)
	t.Cleanup(func() { os.Setenv("XDG_STATE_HOME", orig) })

	// Missing file: Load returns zero State with SchemaVersion set.
	s0, err := Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if s0.Theme != "" {
		t.Errorf("empty Load Theme = %q, want empty", s0.Theme)
	}
	if s0.SchemaVersion != SchemaVersion {
		t.Errorf("empty Load SchemaVersion = %d, want %d", s0.SchemaVersion, SchemaVersion)
	}

	// Save a populated state, then Load and compare.
	in := State{
		Theme:            "osaka-jade",
		PreviousTheme:    "gruvbox-material",
		Wallpaper:        "/tmp/wallpaper.jpg",
		WallpaperByTheme: map[string]string{"osaka-jade": "/tmp/wallpaper.jpg"},
	}
	if err := Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load()
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if out.Theme != "osaka-jade" || out.PreviousTheme != "gruvbox-material" {
		t.Errorf("theme/previous mismatch: %+v", out)
	}
	if out.WallpaperByTheme["osaka-jade"] != "/tmp/wallpaper.jpg" {
		t.Errorf("wallpaper map lost: %+v", out.WallpaperByTheme)
	}
	if out.ChangedAt == "" {
		t.Error("Save should populate ChangedAt")
	}
}

// TestSetCurrentSwapsAtomically verifies the atomic symlink swap: after
// SetCurrent, both state.json and the `current` symlink point at the
// new theme. Also asserts state.PreviousTheme captures the old theme.
func TestSetCurrentSwapsAtomically(t *testing.T) {
	orig := os.Getenv("XDG_STATE_HOME")
	tmp := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmp)
	t.Cleanup(func() { os.Setenv("XDG_STATE_HOME", orig) })

	// Build two fake theme dirs with derived/ subdirs.
	makeTheme := func(name string) string {
		dir := filepath.Join(tmp, "themes", name)
		if err := os.MkdirAll(filepath.Join(dir, "derived"), 0o755); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	oskDir := makeTheme("osaka-jade")
	gruvDir := makeTheme("gruvbox-material")

	// First switch: osaka-jade.
	if err := SetCurrent("osaka-jade", oskDir); err != nil {
		t.Fatalf("SetCurrent(osaka-jade): %v", err)
	}
	if got := CurrentTarget(); got != filepath.Join(oskDir, "derived") {
		t.Errorf("current -> %q, want %q", got, filepath.Join(oskDir, "derived"))
	}
	s, _ := Load()
	if s.Theme != "osaka-jade" || s.PreviousTheme != "" {
		t.Errorf("state after first switch: %+v", s)
	}

	// Second switch: gruvbox — previous should be osaka-jade.
	if err := SetCurrent("gruvbox-material", gruvDir); err != nil {
		t.Fatalf("SetCurrent(gruvbox-material): %v", err)
	}
	s, _ = Load()
	if s.Theme != "gruvbox-material" || s.PreviousTheme != "osaka-jade" {
		t.Errorf("state after swap: %+v", s)
	}
	if got := CurrentTarget(); got != filepath.Join(gruvDir, "derived") {
		t.Errorf("current after swap = %q", got)
	}
}

// TestSetCurrentRejectsMissingDerived documents that SetCurrent refuses
// to point `current` at a theme dir without derived/. Prevents a swap
// that would leave the user's shared configs pointing at nothing.
func TestSetCurrentRejectsMissingDerived(t *testing.T) {
	orig := os.Getenv("XDG_STATE_HOME")
	tmp := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmp)
	t.Cleanup(func() { os.Setenv("XDG_STATE_HOME", orig) })

	dir := filepath.Join(tmp, "themes", "unbuilt")
	os.MkdirAll(dir, 0o755) // no derived/
	if err := SetCurrent("unbuilt", dir); err == nil {
		t.Fatal("expected error for theme with no derived/")
	}
}
