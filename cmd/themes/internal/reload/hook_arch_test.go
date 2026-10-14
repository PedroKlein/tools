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

func TestArchHooks(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux desktop")
	}

	for _, tc := range []struct {
		name, appearance             string
		session, configured, preview bool
	}{
		{"dark", "dark", true, true, false},
		{"light", "light", true, true, false},
		{"headless", "dark", false, true, false},
		{"other-desktop", "dark", true, false, false},
		{"preview", "dark", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, theme := archHookFixture(t, tc.appearance, tc.session, tc.configured)
			skip := archHookSkipList(t)

			results := reload.RunAll(context.Background(), theme, reload.Options{SkipHooks: skip, SkipUserHooks: true, LiveApply: &tc.preview})
			for _, r := range results {
				if r.Err != nil {
					t.Fatalf("%s: %v", r.Name, r.Err)
				}
			}

			file, err := root.Open("log")
			if err != nil {
				t.Fatal(err)
			}

			data, err := io.ReadAll(file)
			_ = file.Close()

			if err != nil {
				t.Fatal(err)
			}

			checkArchCommands(t, string(data), tc.appearance, tc.session && tc.configured, tc.preview)
		})
	}
}

func archHookSkipList(t *testing.T) map[string]bool {
	t.Helper()

	skip := map[string]bool{}
	found := map[string]bool{}

	for _, h := range reload.Registry() {
		if h.Name == "hyprland" || h.Name == "waybar" || h.Name == "linux-system" {
			found[h.Name] = true
		} else {
			skip[h.Name] = true
		}
	}

	for _, name := range []string{"hyprland", "waybar", "linux-system"} {
		if !found[name] {
			t.Fatalf("%s hook missing", name)
		}
	}

	return skip
}

func archHookFixture(t *testing.T, appearance string, session, configured bool) (*os.Root, string) {
	t.Helper()
	dir := t.TempDir()

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = root.Close() })

	for _, p := range []string{"config/themes/sample/derived", "bin", "config/arch/waybar"} {
		if err := os.MkdirAll(filepath.Join(dir, p), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	for _, p := range []string{"log", "config/themes/sample/derived/hyprland.lua", "config/themes/sample/derived/waybar.css"} {
		if err := os.WriteFile(filepath.Join(dir, p), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if configured {
		if err := os.WriteFile(filepath.Join(dir, "config/arch/waybar/style.css"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	manifest := `{"name":"sample","appearance":"` + appearance + `","palette":{"ansi":["#000000","#111111","#222222","#333333","#444444","#555555","#666666","#777777","#888888","#999999","#aaaaaa","#bbbbbb","#cccccc","#dddddd","#eeeeee","#ffffff"],"semantic":{"bg":"#000000","fg":"#ffffff","muted":"#888888","accent":"#111111","error":"#222222","warning":"#333333","ok":"#444444"}}}`
	if err := os.WriteFile(filepath.Join(dir, "config/themes/sample/theme.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	executableMode := os.FileMode(0o600) | 0o100

	for _, cmd := range []string{"hyprctl", "pkill", "gsettings"} {
		script := "#!/bin/sh\nprintf '%s %s\\n' '" + cmd + "' \"$*\" >> \"$ARCH_TEST_LOG\"\n"
		if err := os.WriteFile(filepath.Join(dir, "bin", cmd), []byte(script), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := os.Chmod(filepath.Join(dir, "bin", cmd), executableMode); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("PATH", filepath.Join(dir, "bin"))
	t.Setenv("ARCH_TEST_LOG", filepath.Join(dir, "log"))
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")

	if session {
		t.Setenv("WAYLAND_DISPLAY", "wayland-test")
		t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test")
	}

	return root, filepath.Join(dir, "config", "themes", "sample")
}

func checkArchCommands(t *testing.T, out, appearance string, active, preview bool) {
	t.Helper()

	if !active {
		if out != "" {
			t.Fatalf("unexpected commands: %s", out)
		}

		return
	}

	for _, want := range []string{"hyprctl reload", "pkill -SIGUSR2 -x waybar"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}

	if preview {
		if strings.Contains(out, "gsettings") {
			t.Fatalf("preview changed system: %s", out)
		}

		return
	}

	scheme, gtk := "prefer-dark", "Adwaita-dark"
	if appearance == "light" {
		scheme, gtk = "prefer-light", "Adwaita"
	}

	for _, want := range []string{
		"gsettings set org.gnome.desktop.interface color-scheme " + scheme,
		"gsettings set org.gnome.desktop.interface gtk-theme " + gtk,
		"gsettings set org.gnome.desktop.interface icon-theme Adwaita",
		"gsettings set org.gnome.desktop.interface cursor-theme Adwaita",
		"gsettings set org.gnome.desktop.interface cursor-size 24",
		"gsettings set org.gnome.desktop.interface font-name Adwaita Sans 11",
		"gsettings set org.gnome.desktop.interface monospace-font-name JetBrainsMono Nerd Font 11",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}
