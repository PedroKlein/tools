package reload

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var ompAgentDir = func() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".omp", "agent")
}

func hookOMP(_ context.Context, themeDir string) error {
	src := filepath.Join(themeDir, "derived", "omp.json")
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("hookOMP: stat %s: %w", src, err)
	}

	raw, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("hookOMP: read %s: %w", src, err)
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("hookOMP: parse %s: %w", src, err)
	}
	doc["name"] = "current"

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("hookOMP: marshal: %w", err)
	}
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}

	agentDir := ompAgentDir()
	if agentDir == "" {
		return nil
	}
	if _, err := os.Stat(agentDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("hookOMP: stat agent dir: %w", err)
	}

	themesDir := filepath.Join(agentDir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return fmt.Errorf("hookOMP: mkdir themes dir: %w", err)
	}
	dst := filepath.Join(themesDir, "current.json")
	if err := writeAtomic(dst, out); err != nil {
		return fmt.Errorf("hookOMP: write current theme: %w", err)
	}
	return nil
}
