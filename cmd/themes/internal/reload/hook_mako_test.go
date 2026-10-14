package reload_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/cmd/themes/internal/reload"
)

func TestMakoReload(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux desktop")
	}

	for _, name := range []string{"commit", "preview", "cancel", "headless", "other-desktop", "absent", "fallback", "failure"} {
		t.Run(name, func(t *testing.T) {
			base, theme := makoHookFixture(t, name)
			preview := name == "preview"

			results := reload.RunAll(context.Background(), theme, reload.Options{SkipHooks: makoSkipList(t), SkipUserHooks: true, LiveApply: &preview})
			if len(results) != 1 || (results[0].Err != nil) != (name == "failure") {
				t.Fatalf("unexpected reload results: %+v", results)
			}

			data, err := os.ReadFile(filepath.Join(filepath.Dir(base), "log"))
			if err != nil {
				t.Fatal(err)
			}

			active := name != "headless" && name != "other-desktop" && name != "absent"
			if strings.Contains(string(data), "desktop-colors.sh mako --reload") != active {
				t.Fatalf("unexpected command trace: %s", data)
			}
		})
	}
}

func makoHookFixture(t *testing.T, name string) (string, string) {
	t.Helper()

	_, theme := archHookFixture(t, "dark", name != "headless", name != "other-desktop")
	base := filepath.Dir(filepath.Dir(theme))

	for _, path := range []string{filepath.Join(base, "arch", "desktop-colors.sh"), filepath.Join(theme, "derived", "mako.conf")} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if name == "fallback" {
		if err := os.Remove(filepath.Join(theme, "derived", "mako.conf")); err != nil {
			t.Fatal(err)
		}
	}

	bin := filepath.Join(filepath.Dir(base), "bin")
	if name != "absent" {
		writeMakoExecutable(t, filepath.Join(bin, "makoctl"), "#!/bin/sh\nexit 0\n")
	}

	exit := "0"
	if name == "failure" {
		exit = "1"
	}

	writeMakoExecutable(t, filepath.Join(bin, "bash"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$ARCH_TEST_LOG\"\nexit "+exit+"\n")

	return base, theme
}

func writeMakoExecutable(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(path, os.FileMode(0o600)|0o100); err != nil {
		t.Fatal(err)
	}
}

func makoSkipList(t *testing.T) map[string]bool {
	t.Helper()

	skip := map[string]bool{}
	found := false

	for _, hook := range reload.Registry() {
		if hook.Name == "mako" {
			found = true
		} else {
			skip[hook.Name] = true
		}
	}

	if !found {
		t.Fatal("notification reload hook missing")
	}

	return skip
}
