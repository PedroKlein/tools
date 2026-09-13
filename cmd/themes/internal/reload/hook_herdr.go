package reload

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

var herdrConfigPath = func() string {
	if path := os.Getenv("HERDR_CONFIG_PATH"); path != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "herdr", "config.toml")
}

var runHerdrReload = func(ctx context.Context) error {
	if _, err := exec.LookPath("herdr"); err != nil {
		return nil
	}
	cmd := exec.CommandContext(ctx, "herdr", "server", "reload-config")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, msg)
}

func hookHerdr(ctx context.Context, themeDir string) error {
	theme, err := palette.Load(themeDir)
	if err != nil {
		return fmt.Errorf("hookHerdr: load theme: %w", err)
	}

	configPath := herdrConfigPath()
	if configPath == "" {
		return nil
	}
	raw, fromSeed, err := readHerdrConfig(configPath)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	out := renderHerdrConfig(string(raw), theme.Palette.Semantic.Accent)
	if fromSeed || out != string(raw) {
		if err := writeAtomic(configPath, []byte(out)); err != nil {
			return fmt.Errorf("hookHerdr: write config: %w", err)
		}
	}
	if err := runHerdrReload(ctx); err != nil {
		return fmt.Errorf("hookHerdr: reload config: %w", err)
	}
	return nil
}

func readHerdrConfig(configPath string) ([]byte, bool, error) {
	raw, err := os.ReadFile(configPath)
	if err == nil {
		return raw, false, nil
	}
	if !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("hookHerdr: read config: %w", err)
	}

	seedPath := filepath.Join(filepath.Dir(configPath), "config.seed.toml")
	seed, seedErr := os.ReadFile(seedPath)
	if seedErr == nil {
		return seed, true, nil
	}
	if os.IsNotExist(seedErr) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("hookHerdr: read seed config: %w", seedErr)
}

func renderHerdrConfig(input, accent string) string {
	lines := splitConfigLines(input)
	lines = upsertTomlKey(lines, "theme", "name", strconv.Quote("terminal"))
	lines = upsertTomlKey(lines, "theme.custom", "panel_bg", strconv.Quote("reset"))
	lines = upsertTomlKey(lines, "theme.custom", "accent", strconv.Quote(accent))
	lines = upsertTomlKey(lines, "ui", "accent", strconv.Quote(accent))
	return strings.Join(lines, "\n") + "\n"
}

func splitConfigLines(input string) []string {
	input = strings.TrimRight(input, "\n")
	if input == "" {
		return nil
	}
	return strings.Split(input, "\n")
}

func upsertTomlKey(lines []string, table, key, value string) []string {
	tableIndex := findTomlTable(lines, table)
	if tableIndex == -1 {
		insertAt := tomlTableInsertIndex(lines, table)
		block := []string{"[" + table + "]", key + " = " + value, ""}
		return insertLines(lines, insertAt, block)
	}

	end := nextTomlTable(lines, tableIndex+1)
	for i := tableIndex + 1; i < end; i++ {
		if isTomlKey(lines[i], key) {
			lines[i] = key + " = " + value
			return lines
		}
	}
	return insertLines(lines, end, []string{key + " = " + value})
}

func findTomlTable(lines []string, table string) int {
	want := "[" + table + "]"
	for i, line := range lines {
		if strings.TrimSpace(line) == want {
			return i
		}
	}
	return -1
}

func tomlTableInsertIndex(lines []string, table string) int {
	if table == "ui" {
		if idx := firstTomlSubtable(lines, "ui"); idx != -1 {
			return idx
		}
	}
	if parent, _, ok := strings.Cut(table, "."); ok {
		if idx := findTomlTable(lines, parent); idx != -1 {
			return nextTomlTable(lines, idx+1)
		}
		if idx := firstTomlSubtable(lines, parent); idx != -1 {
			return idx
		}
	}
	return len(lines)
}

func firstTomlSubtable(lines []string, parent string) int {
	prefix := "[" + parent + "."
	arrayPrefix := "[[" + parent + "."
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) || strings.HasPrefix(trimmed, arrayPrefix) {
			return i
		}
	}
	return -1
}

func nextTomlTable(lines []string, start int) int {
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
			return i
		}
	}
	return len(lines)
}

func isTomlKey(line, key string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") || !strings.HasPrefix(trimmed, key) {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, key))
	return strings.HasPrefix(rest, "=")
}

func insertLines(lines []string, index int, block []string) []string {
	if index < 0 {
		index = 0
	}
	if index > len(lines) {
		index = len(lines)
	}
	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:index]...)
	out = append(out, block...)
	out = append(out, lines[index:]...)
	return out
}
