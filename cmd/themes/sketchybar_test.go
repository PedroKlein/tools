package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewSketchybarUsesThemeRoot(t *testing.T) {
	tmp := t.TempDir()
	origConfig := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Cleanup(func() { os.Setenv("XDG_CONFIG_HOME", origConfig) })

	themeRoot := filepath.Join(themesRoot(), "osaka-jade")
	if err := os.MkdirAll(themeRoot, 0o750); err != nil {
		t.Fatal(err)
	}

	origHook := previewSketchybarHook

	var gotThemeRoot string
	previewSketchybarHook = func(themeRoot string) error {
		gotThemeRoot = themeRoot
		return nil
	}

	t.Cleanup(func() { previewSketchybarHook = origHook })

	if err := PreviewSketchybar("osaka-jade"); err != nil {
		t.Fatalf("PreviewSketchybar: %v", err)
	}
	if gotThemeRoot != themeRoot {
		t.Fatalf("preview hook got theme root %q, want %q", gotThemeRoot, themeRoot)
	}
}

func TestPickerSketchybarPreviewIgnoresStaleMessage(t *testing.T) {
	m := &pickerModel{
		themes:               []ThemeInfo{{Name: "osaka-jade"}},
		liveApply:            true,
		sketchybarPreviewSeq: 2,
	}

	_, cmd := m.handleSketchybarPreview(sketchybarPreviewMsg{seq: 1, theme: "osaka-jade"})
	if cmd != nil {
		t.Fatal("stale sketchybar preview message returned a command")
	}
}

func TestPickerSketchybarPreviewRunsForCurrentMessage(t *testing.T) {
	tmp := t.TempDir()
	origConfig := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Cleanup(func() { os.Setenv("XDG_CONFIG_HOME", origConfig) })

	themeRoot := filepath.Join(themesRoot(), "osaka-jade")
	if err := os.MkdirAll(themeRoot, 0o750); err != nil {
		t.Fatal(err)
	}

	origHook := previewSketchybarHook

	var gotThemeRoot string
	previewSketchybarHook = func(themeRoot string) error {
		gotThemeRoot = themeRoot
		return nil
	}

	t.Cleanup(func() { previewSketchybarHook = origHook })

	m := &pickerModel{
		themes:               []ThemeInfo{{Name: "osaka-jade"}},
		liveApply:            true,
		sketchybarPreviewSeq: 1,
	}

	_, cmd := m.handleSketchybarPreview(sketchybarPreviewMsg{seq: 1, theme: "osaka-jade"})
	if cmd == nil {
		t.Fatal("current sketchybar preview message returned nil command")
	}
	msg := cmd()

	done, ok := msg.(sketchybarPreviewDoneMsg)
	if !ok {
		t.Fatalf("preview command returned %T, want sketchybarPreviewDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("preview command error: %v", done.err)
	}
	if gotThemeRoot != themeRoot {
		t.Fatalf("preview hook got theme root %q, want %q", gotThemeRoot, themeRoot)
	}
}
