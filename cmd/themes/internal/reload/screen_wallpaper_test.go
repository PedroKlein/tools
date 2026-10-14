package reload_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/cmd/themes/internal/reload"
	"github.com/PedroKlein/tools/cmd/themes/internal/state"
)

func TestCommittedWallpaperFallback(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux wallpaper setter")
	}

	for _, name := range []string{"missing-selection", "directory-selection"} {
		t.Run(name, func(t *testing.T) {
			_, theme := archHookFixture(t, "dark", true, true)
			background := filepath.Join(theme, "backgrounds", "valid image.png")
			if err := os.MkdirAll(filepath.Dir(background), 0o700); err != nil {
				t.Fatal(err)
			}

			writeScreenFixture(t, background, "")
			bin := os.Getenv("PATH")
			prepareWallpaperSetter(t, bin)

			selected := filepath.Join(theme, "deleted.png")
			if name == "directory-selection" {
				selected = theme
			}

			if err := state.Save(state.State{Theme: "sample", Wallpaper: selected}); err != nil {
				t.Fatal(err)
			}

			skip := map[string]bool{}

			for _, hook := range reload.Registry() {
				if hook.Name != "wallpaper" {
					skip[hook.Name] = true
				}
			}

			for _, result := range reload.RunAll(context.Background(), theme, reload.Options{SkipHooks: skip, SkipUserHooks: true}) {
				if result.Err != nil {
					t.Fatal(result.Err)
				}
			}

			log := os.Getenv("ARCH_TEST_LOG")
			out := readScreenFixture(t, filepath.Dir(log), filepath.Base(log))
			if strings.TrimSpace(string(out)) != background {
				t.Fatalf("wallpaper = %q, want %q", out, background)
			}
		})
	}
}

func prepareWallpaperSetter(t *testing.T, bin string) {
	t.Helper()

	writeScreenFixture(t, filepath.Join(bin, "awww"), "#!/bin/sh\nprintf '%s' \"$2\" > \"$ARCH_TEST_LOG\"\n")

	root, err := os.OpenRoot(bin)
	if err != nil {
		t.Fatal(err)
	}

	defer root.Close()

	file, err := root.Open("awww")
	if err != nil {
		t.Fatal(err)
	}

	defer file.Close()

	if err := file.Chmod(os.FileMode(0o600) | 0o100); err != nil {
		t.Fatal(err)
	}
}
