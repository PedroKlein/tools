package reload

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	xdgstate "github.com/PedroKlein/tools/cmd/themes/internal/state"
)

func hookScreens(ctx context.Context, themeDir string) error {
	if !archDesktopActive() || isLiveApply(ctx) {
		return nil
	}

	saved, err := xdgstate.Load()
	if err != nil {
		return fmt.Errorf("read screen theme state: %w", err)
	}

	if saved.Theme != themeName(themeDir) {
		return nil
	}

	root, err := os.OpenRoot(filepath.Join(themeDir, "derived"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("open screen palette directory: %w", err)
	}

	defer func() { _ = root.Close() }()

	colors, err := readScreenPalette(root, "hyprlock.conf")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("read lock palette: %w", err)
	}

	wallpaper := resolveWallpaper(themeDir)
	if strings.ContainsAny(wallpaper, "\n\r#$") {
		return errors.New("wallpaper path contains unsupported lock config characters")
	}

	data, err := loginSnapshot(root, wallpaper)
	if err != nil {
		return err
	}

	if err = ctx.Err(); err != nil {
		return fmt.Errorf("prepare screen state: %w", err)
	}

	dir := filepath.Join(filepath.Dir(xdgstate.StatePath()), "screens")
	if mkdirErr := os.MkdirAll(dir, 0o700); mkdirErr != nil {
		return fmt.Errorf("create screen state directory: %w", mkdirErr)
	}

	colors = append(colors, []byte("\n$screen_wallpaper = "+wallpaper+"\n")...)
	if writeErr := writeAtomic(filepath.Join(dir, "hyprlock.conf"), colors); writeErr != nil {
		return fmt.Errorf("write lock theme: %w", writeErr)
	}

	if err := writeAtomic(filepath.Join(dir, "sddm.json"), data); err != nil {
		return fmt.Errorf("write login snapshot: %w", err)
	}

	return syncScreens(ctx, data, wallpaper, saved.ChangedAt)
}

func loginSnapshot(root *os.Root, wallpaper string) ([]byte, error) {
	data, err := readScreenPalette(root, "sddm.json")
	if err != nil {
		return nil, fmt.Errorf("read login palette: %w", err)
	}

	var snapshot map[string]string
	if parseErr := json.Unmarshal(data, &snapshot); parseErr != nil {
		return nil, fmt.Errorf("parse login palette: %w", parseErr)
	}

	if snapshot == nil {
		return nil, errors.New("login palette must be a JSON object")
	}

	snapshot["wallpaper"] = wallpaper

	data, err = json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encode login snapshot: %w", err)
	}

	return data, nil
}

func readScreenPalette(root *os.Root, name string) ([]byte, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open palette: %w", err)
	}

	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, 64*1024+1))
	if err != nil {
		return nil, fmt.Errorf("read palette: %w", err)
	}

	if len(data) > 64*1024 {
		return nil, errors.New("screen palette exceeds 64 KiB")
	}

	return data, nil
}
