package reload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func hookSwayOSD(ctx context.Context, _ string) error {
	if !archDesktopActive() {
		return nil
	}

	unit := filepath.Join(filepath.Dir(themesRoot()), "systemd", "user", "swayosd.service")
	if _, err := os.Stat(unit); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("read audio feedback unit: %w", err)
	}

	script := filepath.Join(filepath.Dir(themesRoot()), "arch", "desktop-colors.sh")
	if _, err := os.Stat(script); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("read audio feedback theme input: %w", err)
	}

	return runCommand(ctx, Hook{Cmd: "bash", Args: []string{script, "swayosd", "--reload"}}, nil)
}
