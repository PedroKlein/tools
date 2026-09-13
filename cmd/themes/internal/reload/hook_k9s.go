package reload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

var k9sConfigDir = func() string {
	if dir := os.Getenv("K9S_CONFIG_DIR"); dir != "" {
		return dir
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "k9s")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "k9s")
}

func hookK9s(_ context.Context, themeDir string) error {
	src := filepath.Join(themeDir, "derived", "k9s.yaml")
	payload, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("hookK9s: read skin: %w", err)
	}

	configDir := k9sConfigDir()
	if configDir == "" {
		return nil
	}
	skinsDir := filepath.Join(configDir, "skins")
	if err := os.MkdirAll(skinsDir, 0o755); err != nil {
		return fmt.Errorf("hookK9s: mkdir skins dir: %w", err)
	}
	if err := writeAtomic(filepath.Join(skinsDir, "current.yaml"), payload); err != nil {
		return fmt.Errorf("hookK9s: write current skin: %w", err)
	}
	return nil
}
