package reload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

func openRGBActive(ctx context.Context) (bool, error) {
	if !archDesktopActive() {
		return false, nil
	}

	service := filepath.Join(filepath.Dir(themesRoot()), "systemd", "user", "openrgb-theme.service")
	if _, err := os.Stat(service); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("read RGB service: %w", err)
	}

	if _, err := exec.LookPath("openrgb"); errors.Is(err, exec.ErrNotFound) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("find OpenRGB: %w", err)
	}

	if _, err := exec.LookPath("systemctl"); errors.Is(err, exec.ErrNotFound) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("find RGB service manager: %w", err)
	}

	if err := exec.CommandContext(ctx, "systemctl", "--user", "is-active", "--quiet", "openrgb-theme.service").Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && (exit.ExitCode() == 3 || exit.ExitCode() == 4) {
			return false, nil
		}

		return false, fmt.Errorf("check RGB service: %w", err)
	}

	return true, nil
}

func hookOpenRGB(ctx context.Context, themeDir string) error {
	active, err := openRGBActive(ctx)
	if err != nil {
		return err
	}

	if !active {
		return nil
	}

	theme, err := palette.Load(themeDir)
	if err != nil {
		return fmt.Errorf("load RGB accent: %w", err)
	}

	accent := strings.TrimPrefix(theme.Palette.Semantic.Accent, "#")
	if len(accent) == 3 {
		accent = string([]byte{accent[0], accent[0], accent[1], accent[1], accent[2], accent[2]})
	}

	if len(accent) == 8 {
		accent = accent[:6]
	}

	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		return nil
	}

	runtimeRoot, err := os.OpenRoot(runtime)
	if err != nil {
		return fmt.Errorf("open RGB runtime directory: %w", err)
	}
	defer func() { _ = runtimeRoot.Close() }()

	if err := runtimeRoot.Mkdir("openrgb-theme-client", 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("create RGB client directory: %w", err)
	}

	config := filepath.Join(runtime, "openrgb-theme-client")

	settings := []byte("{\"Server\":{\"default_host\":\"127.0.0.1\",\"default_port\":6742,\"all_controllers\":false}}\n")
	if err := writeAtomic(filepath.Join(config, "OpenRGB.json"), settings); err != nil {
		return fmt.Errorf("write RGB client settings: %w", err)
	}

	if err := runCommand(ctx, Hook{Cmd: "openrgb", Args: []string{"--config", config, "--nodetect", "--device", "B760M AORUS ELITE", "--zone", "0", "--mode", "Static", "--color", accent, "--device", "B760M AORUS ELITE", "--zone", "1", "--mode", "Static", "--color", accent}}, nil); err != nil {
		return fmt.Errorf("set RGB accent: %w", err)
	}

	return nil
}
