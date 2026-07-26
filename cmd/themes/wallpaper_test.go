package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	xdgstate "github.com/PedroKlein/tools/cmd/themes/internal/state"
)

func TestSetWallpaperUpdatesStateAndRunsWallpaperHook(t *testing.T) {
	tmp := t.TempDir()
	origConfig := os.Getenv("XDG_CONFIG_HOME")
	origState := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	os.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Cleanup(func() {
		os.Setenv("XDG_CONFIG_HOME", origConfig)
		os.Setenv("XDG_STATE_HOME", origState)
	})

	themeRoot := filepath.Join(themesRoot(), "osaka-jade")
	if err := os.MkdirAll(filepath.Join(themeRoot, "derived"), 0o755); err != nil {
		t.Fatal(err)
	}
	wallpaper := filepath.Join(tmp, "wallpaper.jpg")
	if err := os.WriteFile(wallpaper, []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := xdgstate.Save(xdgstate.State{Theme: "osaka-jade"}); err != nil {
		t.Fatal(err)
	}

	origHook := applyWallpaperHook
	var gotThemeRoot string
	applyWallpaperHook = func(themeRoot string) error {
		gotThemeRoot = themeRoot
		return nil
	}
	t.Cleanup(func() { applyWallpaperHook = origHook })

	if err := SetWallpaper(wallpaper); err != nil {
		t.Fatalf("SetWallpaper: %v", err)
	}
	if gotThemeRoot != themeRoot {
		t.Fatalf("wallpaper hook got theme root %q, want %q", gotThemeRoot, themeRoot)
	}

	state, err := xdgstate.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Wallpaper != wallpaper {
		t.Fatalf("state wallpaper = %q, want %q", state.Wallpaper, wallpaper)
	}
	if state.WallpaperByTheme["osaka-jade"] != wallpaper {
		t.Fatalf("wallpaper_by_theme not updated: %#v", state.WallpaperByTheme)
	}
}

func TestSetWallpaperReturnsWallpaperHookError(t *testing.T) {
	tmp := t.TempDir()
	origConfig := os.Getenv("XDG_CONFIG_HOME")
	origState := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	os.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Cleanup(func() {
		os.Setenv("XDG_CONFIG_HOME", origConfig)
		os.Setenv("XDG_STATE_HOME", origState)
	})

	themeRoot := filepath.Join(themesRoot(), "osaka-jade")
	if err := os.MkdirAll(filepath.Join(themeRoot, "derived"), 0o755); err != nil {
		t.Fatal(err)
	}
	wallpaper := filepath.Join(tmp, "wallpaper.jpg")
	if err := os.WriteFile(wallpaper, []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := xdgstate.Save(xdgstate.State{Theme: "osaka-jade"}); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("setter unavailable")
	origHook := applyWallpaperHook
	applyWallpaperHook = func(_ string) error { return wantErr }
	t.Cleanup(func() { applyWallpaperHook = origHook })

	if err := SetWallpaper(wallpaper); !errors.Is(err, wantErr) {
		t.Fatalf("SetWallpaper error = %v, want %v", err, wantErr)
	}
}

func TestPreviewWallpaperUsesThemeRoot(t *testing.T) {
	tmp := t.TempDir()
	origConfig := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Cleanup(func() { os.Setenv("XDG_CONFIG_HOME", origConfig) })

	themeRoot := filepath.Join(themesRoot(), "osaka-jade")
	if err := os.MkdirAll(themeRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	origHook := previewWallpaperHook
	var gotThemeRoot string
	previewWallpaperHook = func(themeRoot string) error {
		gotThemeRoot = themeRoot
		return nil
	}
	t.Cleanup(func() { previewWallpaperHook = origHook })

	if err := PreviewWallpaper("osaka-jade"); err != nil {
		t.Fatalf("PreviewWallpaper: %v", err)
	}
	if gotThemeRoot != themeRoot {
		t.Fatalf("preview hook got theme root %q, want %q", gotThemeRoot, themeRoot)
	}
}

func TestPickerWallpaperPreviewIgnoresStaleMessage(t *testing.T) {
	m := &pickerModel{
		themes:              []ThemeInfo{{Name: "osaka-jade"}},
		liveApply:           true,
		wallpaperPreviewSeq: 2,
	}

	_, cmd := m.handleWallpaperPreview(wallpaperPreviewMsg{seq: 1, theme: "osaka-jade"})
	if cmd != nil {
		t.Fatal("stale wallpaper preview message returned a command")
	}
}

func TestPickerWallpaperPreviewRunsForCurrentMessage(t *testing.T) {
	tmp := t.TempDir()
	origConfig := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Cleanup(func() { os.Setenv("XDG_CONFIG_HOME", origConfig) })

	themeRoot := filepath.Join(themesRoot(), "osaka-jade")
	if err := os.MkdirAll(themeRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	origHook := previewWallpaperHook
	var gotThemeRoot string
	previewWallpaperHook = func(themeRoot string) error {
		gotThemeRoot = themeRoot
		return nil
	}
	t.Cleanup(func() { previewWallpaperHook = origHook })

	m := &pickerModel{
		themes:              []ThemeInfo{{Name: "osaka-jade"}},
		liveApply:           true,
		wallpaperPreviewSeq: 1,
	}

	_, cmd := m.handleWallpaperPreview(wallpaperPreviewMsg{seq: 1, theme: "osaka-jade"})
	if cmd == nil {
		t.Fatal("current wallpaper preview message returned nil command")
	}
	msg := cmd()
	done, ok := msg.(wallpaperPreviewDoneMsg)
	if !ok {
		t.Fatalf("preview command returned %T, want wallpaperPreviewDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("preview command error: %v", done.err)
	}
	if gotThemeRoot != themeRoot {
		t.Fatalf("preview hook got theme root %q, want %q", gotThemeRoot, themeRoot)
	}
}
