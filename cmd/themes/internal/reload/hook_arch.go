package reload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

func archDesktopActive() bool {
	if os.Getenv("WAYLAND_DISPLAY") == "" || os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return false
	}

	_, err := os.Stat(filepath.Join(filepath.Dir(themesRoot()), "arch", "waybar", "style.css"))

	return err == nil
}

func hookHyprland(ctx context.Context, themeDir string) error {
	if !archDesktopActive() {
		return nil
	}

	_, err := os.Stat(filepath.Join(themeDir, "derived", "hyprland.lua"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("read hyprland theme: %w", err)
	}

	return runCommand(ctx, Hook{Cmd: "hyprctl", Args: []string{"reload"}}, nil)
}

func hookWaybar(ctx context.Context, themeDir string) error {
	if !archDesktopActive() {
		return nil
	}

	_, err := os.Stat(filepath.Join(themeDir, "derived", "waybar.css"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("read waybar theme: %w", err)
	}

	err = runCommand(ctx, Hook{Cmd: "pkill", Args: []string{"-SIGUSR2", "-x", "waybar"}}, nil)

	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return nil
	}

	if err != nil {
		return fmt.Errorf("reload waybar: %w", err)
	}

	return nil
}

func hookLinuxSystem(ctx context.Context, themeDir string) error {
	if !archDesktopActive() {
		return nil
	}

	theme, err := palette.Load(themeDir)
	if err != nil {
		return fmt.Errorf("load linux appearance: %w", err)
	}

	scheme, gtk := "prefer-dark", "Adwaita-dark"
	if theme.Appearance == "light" {
		scheme, gtk = "prefer-light", "Adwaita"
	}

	for _, setting := range []struct{ key, value string }{
		{"color-scheme", scheme},
		{"gtk-theme", gtk},
		{"icon-theme", "Adwaita"},
		{"cursor-theme", "Adwaita"},
		{"cursor-size", "24"},
		{"font-name", "Adwaita Sans 11"},
		{"monospace-font-name", "JetBrainsMono Nerd Font 11"},
	} {
		if err := runCommand(ctx, Hook{Cmd: "gsettings", Args: []string{"set", "org.gnome.desktop.interface", setting.key, setting.value}}, nil); err != nil {
			return fmt.Errorf("set linux appearance: %w", err)
		}
	}

	return nil
}
