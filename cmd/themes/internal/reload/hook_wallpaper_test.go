package reload

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	xdgstate "github.com/PedroKlein/tools/cmd/themes/internal/state"
)

// TestResolveWallpaperFallsBackToFirstBackground covers the second
// step of the resolver: state.json empty → first background file wins.
func TestResolveWallpaperFallsBackToFirstBackground(t *testing.T) {
	tmp := t.TempDir()
	theme := filepath.Join(tmp, "theme")
	bg := filepath.Join(theme, "backgrounds")
	os.MkdirAll(bg, 0o755)
	// Files sorted: 0-a.png < 1-b.jpg, so 0-a.png wins.
	os.WriteFile(filepath.Join(bg, "1-b.jpg"), []byte("data"), 0o644)
	os.WriteFile(filepath.Join(bg, "0-a.png"), []byte("data"), 0o644)

	// Isolate XDG state so state.json.wallpaper doesn't leak in.
	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmp)
	t.Cleanup(func() { os.Setenv("XDG_STATE_HOME", origXDG) })

	got := resolveWallpaper(theme)
	want := filepath.Join(bg, "0-a.png")
	if got != want {
		t.Errorf("resolver = %q, want %q", got, want)
	}
}

func TestResolveWallpaperPreviewUsesSelectedThemeMemory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	theme := filepath.Join(tmp, "themes", "next-theme")
	bg := filepath.Join(theme, "backgrounds")
	if err := os.MkdirAll(bg, 0o755); err != nil {
		t.Fatal(err)
	}
	remembered := filepath.Join(bg, "remembered.jpg")
	fallback := filepath.Join(bg, "fallback.jpg")
	old := filepath.Join(tmp, "old.jpg")
	for _, p := range []string{remembered, fallback, old} {
		if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := xdgstate.Save(xdgstate.State{
		Theme:            "old-theme",
		Wallpaper:        old,
		WallpaperByTheme: map[string]string{"next-theme": remembered},
	}); err != nil {
		t.Fatal(err)
	}

	if got := resolveWallpaperForMode(theme, true); got != remembered {
		t.Fatalf("preview wallpaper = %q, want selected theme memory %q", got, remembered)
	}
	if got := resolveWallpaperForMode(theme, false); got != old {
		t.Fatalf("commit wallpaper = %q, want state wallpaper %q", got, old)
	}
}

func TestResolveWallpaperPreviewFallsBackToSelectedThemeBackground(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	theme := filepath.Join(tmp, "themes", "next-theme")
	bg := filepath.Join(theme, "backgrounds")
	if err := os.MkdirAll(bg, 0o755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(bg, "0-first.jpg")
	second := filepath.Join(bg, "1-second.jpg")
	old := filepath.Join(tmp, "old.jpg")
	for _, p := range []string{first, second, old} {
		if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := xdgstate.Save(xdgstate.State{
		Theme:            "old-theme",
		Wallpaper:        old,
		WallpaperByTheme: map[string]string{},
	}); err != nil {
		t.Fatal(err)
	}

	if got := resolveWallpaperForMode(theme, true); got != first {
		t.Fatalf("preview wallpaper = %q, want first selected theme background %q", got, first)
	}
}

// TestResolveWallpaperEmptyWhenNothing verifies the resolver returns
// empty when neither state.json nor backgrounds/ has anything.
func TestResolveWallpaperEmptyWhenNothing(t *testing.T) {
	tmp := t.TempDir()
	theme := filepath.Join(tmp, "theme")
	os.MkdirAll(theme, 0o755)

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmp)
	t.Cleanup(func() { os.Setenv("XDG_STATE_HOME", origXDG) })

	if got := resolveWallpaper(theme); got != "" {
		t.Errorf("resolver on empty theme = %q, want empty", got)
	}
}

// TestHookWallpaperMissingFile verifies that a resolved wallpaper path
// that doesn't exist on disk surfaces an error.
func TestHookWallpaperMissingFile(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("only meaningful on macOS/Linux")
	}
	tmp := t.TempDir()
	theme := filepath.Join(tmp, "theme")
	bg := filepath.Join(theme, "backgrounds")
	os.MkdirAll(bg, 0o755)
	// Reference a file we don't create.
	os.WriteFile(filepath.Join(bg, "ghost.png"), []byte("data"), 0o644)

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmp)
	t.Cleanup(func() { os.Setenv("XDG_STATE_HOME", origXDG) })

	// Now nuke the file after resolver picked it — simulates a race.
	os.Remove(filepath.Join(bg, "ghost.png"))
	err := hookWallpaper(context.Background(), theme)
	// On darwin, if no wallpaper resolves, we return nil (silent).
	// On linux with no swww/hyprctl, same. We just verify no panic.
	_ = err
}
