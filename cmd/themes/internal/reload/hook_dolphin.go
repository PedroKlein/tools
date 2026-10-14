package reload

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func hookDolphin(ctx context.Context, themeDir string) error {
	if !archDesktopActive() {
		return nil
	}

	if _, err := exec.LookPath("dolphin"); errors.Is(err, exec.ErrNotFound) {
		return nil
	} else if err != nil {
		return fmt.Errorf("find Dolphin: %w", err)
	}

	themeRoot, err := os.OpenRoot(themeDir)
	if err != nil {
		return fmt.Errorf("open Dolphin theme: %w", err)
	}

	defer func() { _ = themeRoot.Close() }()

	colors, err := fs.ReadFile(themeRoot.FS(), "derived/dolphin.colors")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("read Dolphin colors: %w", err)
	}

	colors, err = dolphinFolderIcons(themeDir, colors)
	if err != nil {
		return err
	}

	config := filepath.Dir(themesRoot())

	configRoot, err := os.OpenRoot(config)
	if err != nil {
		return fmt.Errorf("open KDE settings directory: %w", err)
	}

	defer func() { _ = configRoot.Close() }()

	globals := filepath.Join(config, "kdeglobals")

	previous, err := fs.ReadFile(configRoot.FS(), "kdeglobals")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read KDE settings: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("reload Dolphin: %w", err)
	}

	if err := writeAtomic(globals, mergeKDEColors(previous, colors)); err != nil {
		return fmt.Errorf("write KDE colors: %w", err)
	}

	return notifyDolphin(ctx)
}

func notifyDolphin(ctx context.Context) error {
	const zero = "int32:0"

	if err := runCommand(ctx, Hook{Cmd: "dbus-send", Args: []string{"--session", "--type=signal", "/KGlobalSettings", "org.kde.KGlobalSettings.notifyChange", "int32:4", zero}}, nil); err != nil {
		return fmt.Errorf("notify Dolphin appearance: %w", err)
	}

	if err := runCommand(ctx, Hook{Cmd: "dbus-send", Args: []string{"--session", "--type=signal", "/KIconLoader", "org.kde.KIconLoader.iconChanged", zero}}, nil); err != nil {
		return fmt.Errorf("notify Dolphin folder icons: %w", err)
	}

	// Dolphin refreshes cached item pixmaps on palette changes, after the icon loader is current.
	if err := runCommand(ctx, Hook{Cmd: "dbus-send", Args: []string{"--session", "--type=signal", "/KGlobalSettings", "org.kde.KGlobalSettings.notifyChange", zero, zero}}, nil); err != nil {
		return fmt.Errorf("notify Dolphin palette: %w", err)
	}

	return nil
}

func kdeGroups(data []byte) (order []string, groups map[string][]string) {
	order = []string{""}
	groups = map[string][]string{"": nil}
	group := ""

	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			group = trimmed
			if _, found := groups[group]; !found {
				order = append(order, group)
				groups[group] = nil
			}
		} else if trimmed != "" {
			groups[group] = append(groups[group], line)
		}
	}

	return order, groups
}

func mergeKDEColors(previous, generated []byte) []byte {
	order, existing := kdeGroups(previous)

	generatedOrder, updates := kdeGroups(generated)
	for _, group := range generatedOrder {
		if group == "" {
			continue
		}

		if _, found := existing[group]; !found {
			order = append(order, group)
		}

		if strings.HasPrefix(group, "[Colors:") {
			existing[group] = updates[group]
			continue
		}

		keys := map[string]bool{}

		for _, line := range updates[group] {
			if key, _, ok := strings.Cut(line, "="); ok {
				keys[strings.TrimSpace(key)] = true
			}
		}

		retained := []string{}

		for _, line := range existing[group] {
			key, _, ok := strings.Cut(line, "=")
			if !ok || !keys[strings.TrimSpace(key)] {
				retained = append(retained, line)
			}
		}

		existing[group] = append(retained, updates[group]...)
	}

	var out strings.Builder

	for _, group := range order {
		if strings.HasPrefix(group, "[Colors:") && updates[group] == nil {
			continue
		}

		if group != "" {
			out.WriteString(group + "\n")
		}

		for _, line := range existing[group] {
			out.WriteString(line + "\n")
		}

		out.WriteByte('\n')
	}

	return []byte(out.String())
}
