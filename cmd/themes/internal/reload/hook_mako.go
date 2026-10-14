package reload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func hookMako(ctx context.Context, _ string) error {
	if !archDesktopActive() {
		return nil
	}

	if _, err := exec.LookPath("makoctl"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil
		}

		return fmt.Errorf("find notification controller: %w", err)
	}

	script := filepath.Join(filepath.Dir(themesRoot()), "arch", "desktop-colors.sh")
	if _, err := os.Stat(script); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("read notification theme input: %w", err)
	}

	return runCommand(ctx, Hook{Cmd: "bash", Args: []string{script, "mako", "--reload"}}, nil)
}
