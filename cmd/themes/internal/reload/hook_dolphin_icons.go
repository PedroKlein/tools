package reload

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

func dolphinFolderIcons(themeDir string, colors []byte) ([]byte, error) {
	theme, err := palette.Load(themeDir)
	if err != nil {
		return nil, fmt.Errorf("load folder accent: %w", err)
	}

	base := "Papirus-Dark"
	if theme.Appearance == "light" {
		base = "Papirus"
	}

	source := papirusSource()
	if source == "" {
		return colors, nil
	}

	hex := strings.TrimPrefix(theme.Palette.Semantic.Accent2, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}

	if len(hex) == 8 {
		hex = hex[:6]
	}

	hex = strings.ToLower(hex)
	name := "themes-folders-" + base + "-" + hex

	overlay, err := folderIconRoot(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = overlay.Close() }()

	if _, statErr := overlay.Stat("index.theme"); statErr != nil {
		if writeErr := writeFolderIcons(source, overlay, base, hex); writeErr != nil {
			return nil, writeErr
		}
	}

	return []byte(strings.Replace(string(colors), "Theme="+base+"\n", "Theme="+name+"\n", 1)), nil
}

func iconDataHome() string {
	if data := os.Getenv("XDG_DATA_HOME"); data != "" {
		return data
	}

	home, _ := os.UserHomeDir()

	return filepath.Join(home, ".local", "share")
}

func folderIconRoot(name string) (*os.Root, error) {
	filesystem, err := os.OpenRoot("/")
	if err != nil {
		return nil, fmt.Errorf("open icon filesystem: %w", err)
	}
	defer func() { _ = filesystem.Close() }()

	path := ""
	for part := range strings.SplitSeq(strings.TrimPrefix(filepath.Join(iconDataHome(), "icons", name), "/"), "/") {
		path = filepath.Join(path, part)
		if err = filesystem.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("create folder icon directory: %w", err)
		}
	}

	root, err := filesystem.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("open folder icon directory: %w", err)
	}

	return root, nil
}

func papirusSource() string {
	dirs := os.Getenv("XDG_DATA_DIRS")
	if dirs == "" {
		dirs = "/usr/local/share:/usr/share"
	}

	for _, dir := range filepath.SplitList(iconDataHome() + string(os.PathListSeparator) + dirs) {
		source := filepath.Join(dir, "icons", "Papirus")

		root, err := os.OpenRoot(source)
		if err != nil {
			continue
		}

		_, err = root.Stat("index.theme")
		_ = root.Close()

		if err == nil {
			return source
		}
	}

	return ""
}

func writeFolderIcons(source string, overlay *os.Root, base, hex string) error {
	root, err := os.OpenRoot(source)
	if err != nil {
		return fmt.Errorf("open Papirus folders: %w", err)
	}
	defer func() { _ = root.Close() }()

	paths, err := fs.Glob(root.FS(), "*/places/*.svg")
	if err != nil {
		return fmt.Errorf("list Papirus folders: %w", err)
	}

	replacements := strings.NewReplacer("#5294e2", "#"+hex, "#4877b1", folderShade(hex, 0.8), "#1d344f", folderShade(hex, 0.35))

	for _, path := range paths {
		if strings.HasPrefix(filepath.Base(path), "folder-blue") {
			continue
		}

		svg, readErr := fs.ReadFile(root.FS(), path)
		if readErr != nil {
			return fmt.Errorf("read folder artwork: %w", readErr)
		}

		if !strings.Contains(string(svg), "#5294e2") {
			continue
		}

		if err = overlay.Mkdir(filepath.Dir(filepath.Dir(path)), 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("create folder icon size: %w", err)
		}

		if err = overlay.Mkdir(filepath.Dir(path), 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("create folder icon context: %w", err)
		}

		if writeErr := writeFolderFile(overlay, path, []byte(replacements.Replace(string(svg)))); writeErr != nil {
			return writeErr
		}
	}

	return writeFolderIndex(root, overlay, base)
}

func writeFolderIndex(root, overlay *os.Root, base string) error {
	index, err := fs.ReadFile(root.FS(), "index.theme")
	if err != nil {
		return fmt.Errorf("read Papirus index: %w", err)
	}

	lines := []string{"[Icon Theme]", "Name=Theme secondary folders", "Inherits=" + base}

	for line := range strings.SplitSeq(string(index), "\n") {
		if line != "[Icon Theme]" && !strings.HasPrefix(line, "Name=") && !strings.HasPrefix(line, "Inherits=") {
			lines = append(lines, line)
		}
	}

	return writeFolderFile(overlay, "index.theme", []byte(strings.Join(lines, "\n")))
}

func writeFolderFile(root *os.Root, path string, data []byte) error {
	file, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open folder asset: %w", err)
	}

	_, err = file.Write(data)
	closeErr := file.Close()

	if err != nil {
		return fmt.Errorf("write folder asset: %w", err)
	}

	if closeErr != nil {
		return fmt.Errorf("close folder asset: %w", closeErr)
	}

	return nil
}

func folderShade(hex string, factor float64) string {
	rgb, _ := strconv.ParseUint(hex, 16, 32)

	return fmt.Sprintf("#%02x%02x%02x", int(float64((rgb>>16)&255)*factor), int(float64((rgb>>8)&255)*factor), int(float64(rgb&255)*factor))
}
