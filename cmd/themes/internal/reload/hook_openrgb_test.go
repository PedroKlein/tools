package reload_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/cmd/themes/internal/reload"
)

func TestOpenRGBReload(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux desktop")
	}

	for _, name := range []string{"lifecycle", "headless", "unconfigured", "stopped", "failure"} {
		t.Run(name, func(t *testing.T) {
			root, theme := openRGBFixture(t, name)
			skip := map[string]bool{}
			found := false

			for _, hook := range reload.Registry() {
				if hook.Name == "openrgb" {
					found = true
				} else {
					skip[hook.Name] = true
				}
			}

			if !found {
				t.Fatal("OpenRGB hook missing")
			}

			for index, accent := range []string{"#111111", "#aBc", "#111111ff"} {
				setOpenRGBAccent(t, root, accent)

				preview := index != 0

				results := reload.RunAll(context.Background(), theme, reload.Options{SkipHooks: skip, SkipUserHooks: true, LiveApply: &preview})
				if len(results) != 1 || (results[0].Err != nil) != (name == "failure") {
					t.Fatalf("reload results: %+v", results)
				}
			}

			log, err := fs.ReadFile(root.FS(), "log")
			if err != nil {
				t.Fatal(err)
			}

			if name == "lifecycle" || name == "failure" {
				checkOpenRGBUpdates(t, string(log))
			} else if len(log) != 0 {
				t.Fatalf("inactive hook sent commands: %s", log)
			}
		})
	}
}

func checkOpenRGBUpdates(t *testing.T, log string) {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(log), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected three updates: %s", log)
	}

	for index, color := range []string{"111111", "aaBBcc", "111111"} {
		for _, zone := range []string{"0", "1"} {
			want := "--device B760M AORUS ELITE --zone " + zone + " --mode Static --color " + color
			if !strings.Contains(lines[index], want) || !strings.Contains(lines[index], "--nodetect") {
				t.Fatalf("missing %q in RGB update: %s", want, lines[index])
			}
		}
	}
}

func setOpenRGBAccent(t *testing.T, root *os.Root, accent string) {
	t.Helper()

	data, err := fs.ReadFile(root.FS(), "config/themes/sample/theme.json")
	if err != nil {
		t.Fatal(err)
	}

	var manifest struct {
		Name       string `json:"name"`
		Appearance string `json:"appearance"`
		Palette    struct {
			Ansi     []string          `json:"ansi"`
			Semantic map[string]string `json:"semantic"`
		} `json:"palette"`
	}

	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}

	manifest.Palette.Semantic["accent"] = accent

	data, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	if err := writeDolphinFixtureFile(root, "config/themes/sample/theme.json", data); err != nil {
		t.Fatal(err)
	}
}

func openRGBFixture(t *testing.T, name string) (*os.Root, string) {
	t.Helper()
	root, theme := archHookFixture(t, "dark", name != "headless", true)
	config := filepath.Dir(filepath.Dir(theme))
	bin := filepath.Join(filepath.Dir(config), "bin")

	serviceDir := filepath.Join(config, "systemd", "user")
	if err := os.MkdirAll(serviceDir, 0o700); err != nil {
		t.Fatal(err)
	}

	if name != "unconfigured" {
		if err := writeDolphinFixtureFile(root, "config/systemd/user/openrgb-theme.service", nil); err != nil {
			t.Fatal(err)
		}
	}

	runtimeDir := filepath.Join(filepath.Dir(config), "runtime")
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

	exit := "0"
	if name == "stopped" {
		exit = "3"
	}

	writeMakoExecutable(t, filepath.Join(bin, "systemctl"), "#!/bin/sh\nexit "+exit+"\n")

	exit = "0"
	if name == "failure" {
		exit = "1"
	}

	writeMakoExecutable(t, filepath.Join(bin, "openrgb"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$ARCH_TEST_LOG\"\nexit "+exit+"\n")

	return root, theme
}
