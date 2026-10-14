package reload_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/cmd/themes/internal/reload"
)

func TestDolphinReload(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux desktop")
	}

	for _, name := range []string{"lifecycle", "headless", "missing-colors", "missing-dolphin", "failure"} {
		t.Run(name, func(t *testing.T) {
			root, theme := dolphinFixture(t, name)
			skip := dolphinSkipList(t)
			preview := false

			results := reload.RunAll(context.Background(), theme, reload.Options{SkipHooks: skip, SkipUserHooks: true, LiveApply: &preview})
			if len(results) != 1 || (results[0].Err != nil) != (name == "failure") {
				t.Fatalf("reload results: %+v", results)
			}

			data, err := fs.ReadFile(root.FS(), "config/kdeglobals")
			if err != nil {
				t.Fatal(err)
			}

			if name != "lifecycle" && name != "failure" {
				if string(data) != dolphinOriginal {
					t.Fatalf("inactive hook changed config: %s", data)
				}

				return
			}

			checkDolphinSettings(t, data, "1,2,3")

			if name == "lifecycle" {
				checkDolphinPreviewRestore(t, root, theme, skip)
			}
		})
	}
}

const dolphinOriginal = "[General]\nfont=Custom font\n[Colors:View]\nBackgroundNormal=old\n[Icons]\nTheme=old\nSize=32\n[Other]\nValue=keep\n"

func dolphinFixture(t *testing.T, name string) (*os.Root, string) {
	t.Helper()
	root, theme := archHookFixture(t, "dark", name != "headless", true)
	config := filepath.Dir(filepath.Dir(theme))
	t.Setenv("XDG_DATA_HOME", filepath.Join(filepath.Dir(config), "data"))
	t.Setenv("XDG_DATA_DIRS", filepath.Join(filepath.Dir(config), "data-system"))

	bin := filepath.Join(filepath.Dir(config), "bin")
	if name != "missing-dolphin" {
		writeMakoExecutable(t, filepath.Join(bin, "dolphin"), "#!/bin/sh\nexit 0\n")
	}

	exit := "0"
	if name == "failure" {
		exit = "1"
	}

	writeMakoExecutable(t, filepath.Join(bin, "dbus-send"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$ARCH_TEST_LOG\"\nexit "+exit+"\n")

	if err := writeDolphinFixtureFile(root, "config/kdeglobals", []byte(dolphinOriginal)); err != nil {
		t.Fatal(err)
	}

	if name != "missing-colors" {
		writeDolphinColors(t, root, "1,2,3")
	}

	return root, theme
}

func dolphinSkipList(t *testing.T) map[string]bool {
	t.Helper()

	skip := map[string]bool{}
	found := false

	for _, h := range reload.Registry() {
		if h.Name == "dolphin" {
			found = true
		} else {
			skip[h.Name] = true
		}
	}

	if !found {
		t.Fatal("Dolphin hook missing")
	}

	return skip
}

func writeDolphinFixtureFile(root *os.Root, path string, data []byte) error {
	file, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	_, err = file.Write(data)
	closeErr := file.Close()

	if err != nil {
		return err
	}

	return closeErr
}

func writeDolphinColors(t *testing.T, root *os.Root, rgb string) {
	t.Helper()

	colors := "[Colors:View]\nBackgroundNormal=" + rgb + "\n[Icons]\nTheme=Papirus-Dark\n[KDE]\nwidgetStyle=Breeze\n"
	if err := writeDolphinFixtureFile(root, "config/themes/sample/derived/dolphin.colors", []byte(colors)); err != nil {
		t.Fatal(err)
	}
}

func checkDolphinSettings(t *testing.T, data []byte, rgb string) {
	t.Helper()

	for _, want := range []string{"font=Custom font", "Size=32", "Value=keep", "BackgroundNormal=" + rgb, "Theme=Papirus-Dark"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %q: %s", want, data)
		}
	}

	if strings.Contains(string(data), "=old") || strings.Count(string(data), "[Icons]") != 1 {
		t.Fatalf("stale settings or duplicate groups: %s", data)
	}
}

func checkDolphinPreviewRestore(t *testing.T, root *os.Root, theme string, skip map[string]bool) {
	t.Helper()

	preview := true

	for _, rgb := range []string{"4,5,6", "1,2,3"} {
		writeDolphinColors(t, root, rgb)

		results := reload.RunAll(context.Background(), theme, reload.Options{SkipHooks: skip, SkipUserHooks: true, LiveApply: &preview})
		if len(results) != 1 || results[0].Err != nil {
			t.Fatalf("preview/restore: %+v", results)
		}

		data, err := fs.ReadFile(root.FS(), "config/kdeglobals")
		if err != nil {
			t.Fatal(err)
		}

		checkDolphinSettings(t, data, rgb)
	}

	log, err := fs.ReadFile(root.FS(), "log")
	if err != nil {
		t.Fatal(err)
	}

	iconSignal := "/KIconLoader org.kde.KIconLoader.iconChanged int32:0"
	if strings.Count(string(log), iconSignal) != 3 {
		t.Fatalf("icon notifications: %s", log)
	}

	ordered := "--session --type=signal /KGlobalSettings org.kde.KGlobalSettings.notifyChange int32:4 int32:0\n--session --type=signal " + iconSignal + "\n--session --type=signal /KGlobalSettings org.kde.KGlobalSettings.notifyChange int32:0" + " int32:0\n"
	if strings.Count(string(log), ordered) != 3 {
		t.Fatalf("icon refresh must precede palette repaint: %s", log)
	}

	paletteSignal := "org.kde.KGlobalSettings.notifyChange int32:0" + " int32:0"
	if strings.Count(string(log), paletteSignal) != 3 {
		t.Fatalf("palette notifications: %s", log)
	}
}
