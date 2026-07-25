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

// makeThemeWithBg builds a theme dir with derived/ and optional
// backgrounds/ files. Returns the theme dir path.
func makeThemeWithBg(t *testing.T, root, name string, bgs ...string) string {
	t.Helper()
	dir := filepath.Join(root, "themes", name)
	if err := os.MkdirAll(filepath.Join(dir, "derived"), 0o755); err != nil {
		t.Fatal(err)
	}
	if len(bgs) > 0 {
		if err := os.MkdirAll(filepath.Join(dir, "backgrounds"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, bg := range bgs {
			if err := os.WriteFile(filepath.Join(dir, "backgrounds", bg), []byte("png"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dir
}

// TestSetCurrentRefreshesWallpaperFromMap covers d3 case (a):
// WallpaperByTheme[newTheme] is set, so state.Wallpaper picks up that
// value — not the stale wallpaper from the previous theme.
func TestSetCurrentRefreshesWallpaperFromMap(t *testing.T) {
	orig := os.Getenv("XDG_STATE_HOME")
	tmp := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmp)
	t.Cleanup(func() { os.Setenv("XDG_STATE_HOME", orig) })

	aDir := makeThemeWithBg(t, tmp, "a", "a1.png")
	bDir := makeThemeWithBg(t, tmp, "b", "b1.png")

	// Seed state with a active + a wallpaper for both themes.
	if err := SetCurrent("a", aDir); err != nil {
		t.Fatal(err)
	}
	s, _ := Load()
	s.WallpaperByTheme = map[string]string{
		"a": filepath.Join(aDir, "backgrounds", "a1.png"),
		"b": filepath.Join(bDir, "backgrounds", "b1.png"),
	}
	s.Wallpaper = s.WallpaperByTheme["a"]
	if err := Save(*s); err != nil {
		t.Fatal(err)
	}

	// Switch to b — state.Wallpaper should reflect b1.png, not a1.png.
	if err := SetCurrent("b", bDir); err != nil {
		t.Fatal(err)
	}
	s, _ = Load()
	want := filepath.Join(bDir, "backgrounds", "b1.png")
	if s.Wallpaper != want {
		t.Errorf("Wallpaper = %q, want %q (from WallpaperByTheme)", s.Wallpaper, want)
	}
}

// TestSetCurrentRefreshesWallpaperFallsBackToFirstBg covers d3 case (b):
// no WallpaperByTheme entry for the new theme, so state.Wallpaper falls
// back to the first background file in <newDir>/backgrounds/.
func TestSetCurrentRefreshesWallpaperFallsBackToFirstBg(t *testing.T) {
	orig := os.Getenv("XDG_STATE_HOME")
	tmp := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmp)
	t.Cleanup(func() { os.Setenv("XDG_STATE_HOME", orig) })

	aDir := makeThemeWithBg(t, tmp, "a", "a1.png")
	bDir := makeThemeWithBg(t, tmp, "b", "b-first.jpg", "b-second.jpg")

	if err := SetCurrent("a", aDir); err != nil {
		t.Fatal(err)
	}
	s, _ := Load()
	s.Wallpaper = filepath.Join(aDir, "backgrounds", "a1.png")
	s.WallpaperByTheme = map[string]string{
		"a": s.Wallpaper,
	}
	if err := Save(*s); err != nil {
		t.Fatal(err)
	}

	if err := SetCurrent("b", bDir); err != nil {
		t.Fatal(err)
	}
	s, _ = Load()
	want := filepath.Join(bDir, "backgrounds", "b-first.jpg")
	if s.Wallpaper != want {
		t.Errorf("Wallpaper = %q, want %q (first bg in b/backgrounds)", s.Wallpaper, want)
	}
}

// TestSetCurrentPreservesPreviousWallpaperInMap covers d3 case (c):
// switching A → B → A retains A's original wallpaper because the
// switch A → B saved A's wallpaper into WallpaperByTheme["a"] first.
// TestIsDriftedDetectsMismatch covers the D4 root-cause: after a bad
// TUI live-preview ESC, symlink and state.json can silently diverge.
// IsDrifted() must catch this so runApply can re-align.
func TestIsDriftedDetectsMismatch(t *testing.T) {
	orig := os.Getenv("XDG_STATE_HOME")
	tmp := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmp)
	t.Cleanup(func() { os.Setenv("XDG_STATE_HOME", orig) })

	aDir := makeThemeWithBg(t, tmp, "a")
	bDir := makeThemeWithBg(t, tmp, "b")

	// Fresh install: no state, no symlink — not drifted.
	if IsDrifted() {
		t.Fatal("fresh install: IsDrifted should be false")
	}

	// Aligned state: IsDrifted false.
	if err := SetCurrent("a", aDir); err != nil {
		t.Fatal(err)
	}
	if IsDrifted() {
		t.Fatal("aligned: IsDrifted should be false")
	}

	// Skew: rewrite state.json.Theme without touching the symlink.
	s, _ := Load()
	s.Theme = "b"
	if err := Save(*s); err != nil {
		t.Fatal(err)
	}
	if !IsDrifted() {
		t.Fatal("skewed: IsDrifted should be true (state says b, symlink still points at a)")
	}

	// Realign: SetCurrent(b) fixes both sides.
	if err := SetCurrent("b", bDir); err != nil {
		t.Fatal(err)
	}
	if IsDrifted() {
		t.Fatal("re-aligned via SetCurrent: IsDrifted should be false")
	}
}

func TestSetCurrentPreservesPreviousWallpaperInMap(t *testing.T) {
	orig := os.Getenv("XDG_STATE_HOME")
	tmp := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmp)
	t.Cleanup(func() { os.Setenv("XDG_STATE_HOME", orig) })

	aDir := makeThemeWithBg(t, tmp, "a", "a-picked.png", "a-first.jpg")
	bDir := makeThemeWithBg(t, tmp, "b", "b1.png")

	if err := SetCurrent("a", aDir); err != nil {
		t.Fatal(err)
	}
	// User picks a NON-first wallpaper for a.
	aPicked := filepath.Join(aDir, "backgrounds", "a-picked.png")
	s, _ := Load()
	s.Wallpaper = aPicked
	if s.WallpaperByTheme == nil {
		s.WallpaperByTheme = map[string]string{}
	}
	s.WallpaperByTheme["a"] = aPicked
	if err := Save(*s); err != nil {
		t.Fatal(err)
	}

	// A → B. B has no map entry; falls back to first bg.
	if err := SetCurrent("b", bDir); err != nil {
		t.Fatal(err)
	}
	s, _ = Load()
	if got := s.WallpaperByTheme["a"]; got != aPicked {
		t.Errorf("WallpaperByTheme[a] = %q, want %q (preserved across switch)", got, aPicked)
	}

	// B → A. State.Wallpaper should restore aPicked from the map.
	if err := SetCurrent("a", aDir); err != nil {
		t.Fatal(err)
	}
	s, _ = Load()
	if s.Wallpaper != aPicked {
		t.Errorf("Wallpaper after B→A = %q, want %q", s.Wallpaper, aPicked)
	}
}
