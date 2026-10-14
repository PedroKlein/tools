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

func TestAudioFeedbackReload(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux desktop")
	}

	for _, name := range []string{"commit", "preview", "headless", "other-desktop", "unconfigured", "missing-wrapper", "failure"} {
		t.Run(name, func(t *testing.T) {
			base, theme := audioFeedbackFixture(t, name)
			preview := name == "preview"
			results := reload.RunAll(context.Background(), theme, reload.Options{SkipHooks: audioFeedbackSkipList(t), SkipUserHooks: true, LiveApply: &preview})
			if preview {
				if len(results) != 0 {
					t.Fatalf("preview restarted feedback: %+v", results)
				}
			} else if len(results) != 1 || (results[0].Err != nil) != (name == "failure") {
				t.Fatalf("unexpected results: %+v", results)
			}

			data, err := os.ReadFile(filepath.Join(filepath.Dir(base), "log"))
			if err != nil {
				t.Fatal(err)
			}

			active := name == "commit" || name == "failure"
			if strings.Contains(string(data), "swayosd --reload") != active {
				t.Fatalf("unexpected command trace: %s", data)
			}
		})
	}
}

func audioFeedbackFixture(t *testing.T, name string) (string, string) {
	t.Helper()

	_, theme := archHookFixture(t, "dark", name != "headless", name != "other-desktop")
	base := filepath.Dir(filepath.Dir(theme))
	unit := filepath.Join(base, "systemd", "user", "swayosd.service")
	if name != "unconfigured" {
		if err := os.MkdirAll(filepath.Dir(unit), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(unit, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	exit := "0"
	if name == "failure" {
		exit = "1"
	}

	script := filepath.Join(base, "arch", "desktop-colors.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
		t.Fatal(err)
	}

	if name != "missing-wrapper" {
		if err := os.WriteFile(script, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	writeMakoExecutable(t, filepath.Join(filepath.Dir(base), "bin", "bash"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$ARCH_TEST_LOG\"\nexit "+exit+"\n")

	return base, theme
}

func audioFeedbackSkipList(t *testing.T) map[string]bool {
	t.Helper()

	skip := map[string]bool{}
	found := false

	for _, hook := range reload.Registry() {
		if hook.Name == "swayosd" {
			found = true
		} else {
			skip[hook.Name] = true
		}
	}

	if !found {
		t.Fatal("audio feedback hook missing")
	}

	return skip
}
