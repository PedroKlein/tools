package reload_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/cmd/themes/internal/reload"
	"github.com/PedroKlein/tools/cmd/themes/internal/state"
)

func TestScreenHookUsesCommittedSelection(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Arch screens")
	}

	for _, mode := range []string{"commit", "preview", "missing-wallpaper", "empty-wallpapers", "stale-theme", "wallpaper-change", "headless", "other-desktop"} {
		t.Run(mode, func(t *testing.T) {
			theme, selected := screenHookFixture(t, mode)
			skip := screenHookSkipList(t)
			preview := mode == "preview"
			runScreenHook(t, theme, skip, preview)

			if preview || mode == "stale-theme" || mode == "headless" || mode == "other-desktop" {
				checkNoScreenSnapshot(t)

				return
			}

			want := selected
			if mode == "missing-wallpaper" {
				want = filepath.Join(theme, "backgrounds", "fallback.png")
			}

			if mode == "empty-wallpapers" {
				want = ""
			}

			checkScreenSnapshot(t, want)
			if mode == "wallpaper-change" {
				changed := filepath.Join(theme, "backgrounds", "fallback.png")
				if err := state.Save(state.State{Theme: "sample", Wallpaper: changed}); err != nil {
					t.Fatal(err)
				}

				runScreenHook(t, theme, skip, false)
				checkScreenSnapshot(t, changed)
			}
		})
	}
}

func TestScreenPreviewAndCancelRetainCommittedSnapshot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Arch screens")
	}

	theme, selected := screenHookFixture(t, "commit")
	skip := screenHookSkipList(t)
	runScreenHook(t, theme, skip, false)

	other := filepath.Join(filepath.Dir(theme), "preview")
	if err := os.MkdirAll(filepath.Join(other, "derived"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := state.SwapCurrentSymlink(other); err != nil {
		t.Fatal(err)
	}

	runScreenHook(t, other, skip, true)
	checkScreenSnapshot(t, selected)
	if err := state.SwapCurrentSymlink(theme); err != nil {
		t.Fatal(err)
	}

	runScreenHook(t, theme, skip, true)
	checkScreenSnapshot(t, selected)
}

func TestScreenHookRejectsInvalidLoginPalette(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Arch screens")
	}

	for _, payload := range []string{"null", "broken json", strings.Repeat("x", 64*1024+1)} {
		t.Run(payload[:min(len(payload), 12)], func(t *testing.T) {
			theme, _ := screenHookFixture(t, "commit")
			writeScreenFixture(t, filepath.Join(theme, "derived", "sddm.json"), payload)
			results := reload.RunAll(context.Background(), theme, reload.Options{SkipHooks: screenHookSkipList(t), SkipUserHooks: true})
			if len(results) != 1 || results[0].Err == nil {
				t.Fatalf("invalid palette accepted: %#v", results)
			}

			checkNoScreenSnapshot(t)
		})
	}
}

func TestScreenPublisherCommitPreviewAndFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Arch screens")
	}

	theme, _ := screenHookFixture(t, "empty-wallpapers")
	bin := os.Getenv("PATH")
	writeScreenFixture(t, filepath.Join(bin, "sddm-theme-sync"), "#!/bin/sh\nexit 0\n")
	writeScreenFixture(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nprintf '%s' \"$*\" > \"$ARCH_TEST_LOG\"\n/bin/cat > \"$ARCH_TEST_LOG.payload\"\nexit 1\n")
	root, err := os.OpenRoot(bin)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	for _, name := range []string{"sudo", "sddm-theme-sync"} {
		file, err := root.Open(name)
		if err != nil {
			t.Fatal(err)
		}

		if err := file.Chmod(0o700); err != nil {
			t.Fatal(err)
		}

		_ = file.Close()
	}

	skip := screenHookSkipList(t)
	runScreenHook(t, theme, skip, true)
	log := os.Getenv("ARCH_TEST_LOG")
	if data := readScreenFixture(t, filepath.Dir(log), filepath.Base(log)); len(data) != 0 {
		t.Fatal("preview invoked the privileged recipient")
	}

	results := reload.RunAll(context.Background(), theme, reload.Options{SkipHooks: skip, SkipUserHooks: true})
	if len(results) != 1 || results[0].Err == nil {
		t.Fatal("sync failure must be observable")
	}

	checkScreenSnapshot(t, "")
	args := readScreenFixture(t, filepath.Dir(log), filepath.Base(log))
	if !strings.HasPrefix(string(args), "-n -- ") || !strings.HasSuffix(string(args), "/sddm-theme-sync") {
		t.Fatalf("unexpected privilege command: %s", args)
	}
	payload := readScreenFixture(t, filepath.Dir(log), filepath.Base(log)+".payload")
	if !strings.Contains(string(payload), `"revision":`) || strings.Contains(string(payload), "wallpaper") {
		t.Fatalf("unsafe publisher payload: %s", payload)
	}
}

func screenHookFixture(t *testing.T, mode string) (string, string) {
	t.Helper()
	_, theme := archHookFixture(t, "dark", mode != "headless", mode != "other-desktop")
	writeScreenFixture(t, filepath.Join(theme, "derived", "sddm.json"), `{"background":"#111111","foreground":"#eeeeee","muted":"#999999","accent":"#aabbcc","error":"#ff0000","font":"Sans"}`)
	writeScreenFixture(t, filepath.Join(theme, "derived", "hyprlock.conf"), "$screen_bg = rgb(111111)\n")
	if err := os.MkdirAll(filepath.Join(theme, "backgrounds"), 0o700); err != nil {
		t.Fatal(err)
	}

	if mode != "empty-wallpapers" {
		writeScreenFixture(t, filepath.Join(theme, "backgrounds", "fallback.png"), "")
	}

	selected := filepath.Join(t.TempDir(), "selected image.png")
	writeScreenFixture(t, selected, "")
	if mode == "missing-wallpaper" || mode == "empty-wallpapers" {
		selected += ".deleted"
	}

	savedTheme := "sample"
	if mode == "stale-theme" {
		savedTheme = "other"
	}

	if err := state.Save(state.State{Theme: savedTheme, Wallpaper: selected}); err != nil {
		t.Fatal(err)
	}

	return theme, selected
}

func writeScreenFixture(t *testing.T, path, value string) {
	t.Helper()

	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}

	defer root.Close()

	file, err := root.OpenFile(filepath.Base(path), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	defer file.Close()

	if _, err := file.WriteString(value); err != nil {
		t.Fatal(err)
	}
}

func screenHookSkipList(t *testing.T) map[string]bool {
	t.Helper()
	skip := map[string]bool{}
	found := false

	for _, hook := range reload.Registry() {
		if hook.Name == "screens" {
			found = true
		} else {
			skip[hook.Name] = true
		}
	}

	if !found {
		t.Fatal("screens hook missing")
	}

	return skip
}

func runScreenHook(t *testing.T, theme string, skip map[string]bool, preview bool) {
	t.Helper()

	for _, result := range reload.RunAll(context.Background(), theme, reload.Options{SkipHooks: skip, SkipUserHooks: true, LiveApply: &preview}) {
		if result.Err != nil {
			t.Fatal(result.Err)
		}
	}
}

func checkScreenSnapshot(t *testing.T, wallpaper string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("XDG_STATE_HOME"), "themes", "screens")
	lock := readScreenFixture(t, dir, "hyprlock.conf")
	if !strings.Contains(string(lock), "$screen_wallpaper = "+wallpaper+"\n") || !strings.Contains(string(lock), "$screen_bg = rgb(111111)") {
		t.Fatalf("unexpected lock state: %s", lock)
	}

	data := readScreenFixture(t, dir, "sddm.json")

	var snapshot struct {
		Wallpaper string `json:"wallpaper"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil || snapshot.Wallpaper != wallpaper {
		t.Fatalf("selected wallpaper = %q, want %q (error %v)", snapshot.Wallpaper, wallpaper, err)
	}
}

func readScreenFixture(t *testing.T, dir, name string) []byte {
	t.Helper()

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer root.Close()

	file, err := root.Open(name)
	if err != nil {
		t.Fatal(err)
	}

	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func checkNoScreenSnapshot(t *testing.T) {
	t.Helper()

	root, err := os.OpenRoot(os.Getenv("XDG_STATE_HOME"))
	if err != nil {
		t.Fatal(err)
	}

	defer root.Close()

	if _, err := root.Stat("themes/screens/hyprlock.conf"); !os.IsNotExist(err) {
		t.Fatalf("inactive hook published lock state: %v", err)
	}
}
