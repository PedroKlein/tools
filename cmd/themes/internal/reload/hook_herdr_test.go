package reload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookHerdrWritesAccentBeforeUISubtables(t *testing.T) {
	tmp := t.TempDir()
	themeDir := writeHerdrTestTheme(t, tmp, "#549E6A")
	configPath := filepath.Join(tmp, "herdr", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		t.Fatal(err)
	}
	input := `onboarding = false

[theme]
name = "terminal"

[theme.custom]
panel_bg = "reset"

# Notifications
[ui.toast]
delivery = "system"
`
	if err := os.WriteFile(configPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	reloads := 0
	stubHerdrHook(t, configPath, func(context.Context) error {
		reloads++
		return nil
	})

	if err := hookHerdr(context.Background(), themeDir); err != nil {
		t.Fatalf("hookHerdr: %v", err)
	}
	if reloads != 1 {
		t.Fatalf("reloads = %d, want 1", reloads)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(raw)
	for _, want := range []string{
		`name = "terminal"`,
		`panel_bg = "reset"`,
		`accent = "#549E6A"`,
		"[ui]\naccent = \"#549E6A\"",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("config missing %q:\n%s", want, out)
		}
	}
	ui := strings.Index(out, "[ui]\n")
	toast := strings.Index(out, "[ui.toast]")
	if ui == -1 || toast == -1 || ui > toast {
		t.Fatalf("[ui] must be before [ui.toast]; got:\n%s", out)
	}
	if strings.Count(out, `accent = "#549E6A"`) != 2 {
		t.Fatalf("expected theme.custom and ui accent only, got:\n%s", out)
	}
}

func TestHookHerdrReplacesStowSymlinkWithRuntimeFile(t *testing.T) {
	tmp := t.TempDir()
	themeDir := writeHerdrTestTheme(t, tmp, "#D8A657")
	configDir := filepath.Join(tmp, "herdr")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	repoConfig := filepath.Join(tmp, "repo", "config.toml")
	if err := os.MkdirAll(filepath.Dir(repoConfig), 0o750); err != nil {
		t.Fatal(err)
	}
	originalRepoConfig := []byte("[theme]\nname = \"terminal\"\n")
	if err := os.WriteFile(repoConfig, originalRepoConfig, 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.Symlink(repoConfig, configPath); err != nil {
		t.Fatal(err)
	}

	stubHerdrHook(t, configPath, func(context.Context) error { return nil })

	if err := hookHerdr(context.Background(), themeDir); err != nil {
		t.Fatalf("hookHerdr: %v", err)
	}
	info, err := os.Lstat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("runtime config is still a symlink")
	}
	repoRaw, err := os.ReadFile(repoConfig)
	if err != nil {
		t.Fatal(err)
	}
	if string(repoRaw) != string(originalRepoConfig) {
		t.Fatalf("repo config was rewritten:\n%s", repoRaw)
	}
	runtimeRaw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(runtimeRaw), `accent = "#D8A657"`) {
		t.Fatalf("runtime config missing theme accent:\n%s", runtimeRaw)
	}
}

func stubHerdrHook(t *testing.T, configPath string, reload func(context.Context) error) {
	t.Helper()
	origConfigPath := herdrConfigPath
	origReload := runHerdrReload
	herdrConfigPath = func() string { return configPath }
	runHerdrReload = reload
	t.Cleanup(func() {
		herdrConfigPath = origConfigPath
		runHerdrReload = origReload
	})
}

func writeHerdrTestTheme(t *testing.T, parent, accent string) string {
	t.Helper()
	themeDir := filepath.Join(parent, "theme")
	if err := os.MkdirAll(themeDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf(`{
  "name": "test-theme",
  "appearance": "dark",
  "palette": {
    "ansi": [
      "#000000", "#ff0000", "#00ff00", "#ffff00",
      "#0000ff", "#ff00ff", "#00ffff", "#ffffff",
      "#111111", "#ff5555", "#55ff55", "#ffff55",
      "#5555ff", "#ff55ff", "#55ffff", "#eeeeee"
    ],
    "semantic": {
      "bg": "#111111",
      "fg": "#eeeeee",
      "muted": "#777777",
      "accent": %q,
      "error": "#ff0000",
      "warning": "#ffff00",
      "ok": "#00ff00"
    }
  }
}
`, accent)
	if err := os.WriteFile(filepath.Join(themeDir, "theme.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return themeDir
}
