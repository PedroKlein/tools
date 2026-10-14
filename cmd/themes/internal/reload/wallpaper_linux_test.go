package reload_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/cmd/themes/internal/reload"
)

func TestWallpaperUsesAwww(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux wallpaper")
	}

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("PATH", dir)
	t.Setenv("WALLPAPER_LOG", filepath.Join(dir, "log"))

	bg := filepath.Join(dir, "theme", "backgrounds")
	if err := os.MkdirAll(bg, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(bg, "wallpaper.png"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$WALLPAPER_LOG\"\n"
	if err := os.WriteFile(filepath.Join(dir, "awww"), []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}

	mode := os.FileMode(0o600) | 0o100
	if err := os.Chmod(filepath.Join(dir, "awww"), mode); err != nil {
		t.Fatal(err)
	}

	skip := map[string]bool{}

	for _, h := range reload.Registry() {
		if h.Name != "wallpaper" {
			skip[h.Name] = true
		}
	}

	results := reload.RunAll(context.Background(), filepath.Join(dir, "theme"), reload.Options{SkipHooks: skip, SkipUserHooks: true})
	for _, r := range results {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	file, err := root.Open("log")
	if err != nil {
		t.Fatal("awww was not invoked:", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	want := "img " + filepath.Join(bg, "wallpaper.png") + " --transition-type any\n"
	if !strings.Contains(string(data), want) {
		t.Fatalf("wallpaper command = %q, want %q", data, want)
	}
}
