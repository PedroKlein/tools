package reload_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/PedroKlein/tools/cmd/themes/internal/reload"
)

func TestCommittedStowWallpaperPublishesBytes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Arch screens")
	}

	for _, oversized := range []bool{false, true} {
		t.Run(map[bool]string{false: "selected-bytes", true: "oversized"}[oversized], func(t *testing.T) {
			theme, source := stowScreenFixture(t)
			if oversized {
				if err := os.Truncate(source, 16*1024*1024+1); err != nil {
					t.Fatal(err)
				}
			}

			results := reload.RunAll(context.Background(), theme, reload.Options{SkipHooks: screenHookSkipList(t), SkipUserHooks: true})
			if len(results) != 1 || (results[0].Err != nil) != oversized {
				t.Fatalf("committed Stow wallpaper publication (oversized=%v): %#v", oversized, results)
			}

			log := os.Getenv("ARCH_TEST_LOG")
			if oversized {
				checkNoLoginPayload(t, log)

				return
			}

			var payload struct {
				Image     []byte `json:"image"`
				Wallpaper string `json:"wallpaper"`
			}

			data := readScreenFixture(t, filepath.Dir(log), filepath.Base(log)+".payload")
			if err := json.Unmarshal(data, &payload); err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(payload.Image, []byte("selected synthetic raster bytes")) || payload.Wallpaper != "" {
				t.Fatalf("publisher must receive selected bytes, not a user path: %#v", payload)
			}
		})
	}
}

func checkNoLoginPayload(t *testing.T, log string) {
	t.Helper()

	root, err := os.OpenRoot(filepath.Dir(log))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if _, err := root.Stat(filepath.Base(log) + ".payload"); !os.IsNotExist(err) {
		t.Fatalf("oversized target invoked publisher: %v", err)
	}
}

func stowScreenFixture(t *testing.T) (string, string) {
	t.Helper()
	theme, selected := screenHookFixture(t, "commit")
	source := filepath.Join(t.TempDir(), "wallpaper.png")
	writeScreenFixture(t, source, "selected synthetic raster bytes")
	if err := os.Remove(selected); err != nil {
		t.Fatal(err)
	}

	relative, err := filepath.Rel(filepath.Dir(selected), source)
	if err != nil {
		t.Fatal(err)
	}

	if err = os.Symlink(relative, selected); err != nil {
		t.Fatal(err)
	}

	bin := os.Getenv("PATH")
	for name, script := range map[string]string{
		"sddm-theme-sync": "#!/bin/sh\nexit 0\n",
		"sudo":            "#!/bin/sh\n/bin/cat > \"$ARCH_TEST_LOG.payload\"\n",
	} {
		path := filepath.Join(bin, name)
		writeScreenFixture(t, path, script)
		file, openErr := os.OpenRoot(bin)
		if openErr != nil {
			t.Fatal(openErr)
		}

		command, openErr := file.Open(name)
		if openErr != nil {
			_ = file.Close()

			t.Fatal(openErr)
		}

		mode := os.FileMode(0o600) | 0o100
		chmodErr := command.Chmod(mode)
		_ = command.Close()
		_ = file.Close()

		if chmodErr != nil {
			t.Fatal(chmodErr)
		}
	}

	return theme, source
}
