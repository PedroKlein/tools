package palette

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveWallpaper picks the wallpaper file to show for theme t.
//
// Layered lookup (first-match wins):
//  1. `override` — explicit user pick from state.json, if it exists on
//     disk (path or bare filename inside <t.Dir>/backgrounds/).
//  2. theme.Wallpapers.Default resolved against <t.Dir>/backgrounds/.
//  3. First entry in theme.Wallpapers.List resolved likewise.
//  4. First .jpg/.png (alphabetical) in <t.Dir>/backgrounds/.
//
// Returns the absolute path and (Wallpapers.Placement || "fill").
// If nothing resolves, returns "" with a non-nil error.
//
// This function is invoked by the macos-system.sh + wallpaper.sh hooks
// (via the CLI). It never touches state itself; the caller decides
// whether to persist the choice.
func ResolveWallpaper(t *Theme, override string) (path, placement string, err error) {
	if t == nil {
		return "", "", fmt.Errorf("ResolveWallpaper: nil theme")
	}
	bgDir := filepath.Join(t.Dir, "backgrounds")
	placement = t.Wallpapers.Placement
	if placement == "" {
		placement = "fill"
	}

	// 1. Explicit override.
	if override != "" {
		if isAbsPath(override) {
			if _, err := os.Stat(override); err == nil {
				return override, placement, nil
			}
		}
		p := filepath.Join(bgDir, override)
		if _, err := os.Stat(p); err == nil {
			return p, placement, nil
		}
	}

	// 2. theme.Wallpapers.Default
	if t.Wallpapers.Default != "" {
		p := filepath.Join(bgDir, t.Wallpapers.Default)
		if _, err := os.Stat(p); err == nil {
			return p, placement, nil
		}
	}

	// 3. theme.Wallpapers.List[0]
	for _, w := range t.Wallpapers.List {
		p := filepath.Join(bgDir, w.File)
		if _, err := os.Stat(p); err == nil {
			return p, placement, nil
		}
	}

	// 4. First image in bgDir.
	entries, err := os.ReadDir(bgDir)
	if err != nil {
		return "", placement, fmt.Errorf("no backgrounds/ dir in %s: %w", t.Dir, err)
	}
	var candidates []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".heic" {
			candidates = append(candidates, name)
		}
	}
	if len(candidates) == 0 {
		return "", placement, fmt.Errorf("no wallpapers in %s", bgDir)
	}
	sort.Strings(candidates)
	return filepath.Join(bgDir, candidates[0]), placement, nil
}

// isAbsPath is filepath.IsAbs, extracted so callers can override for
// unit tests that need to spoof paths.
var isAbsPath = filepath.IsAbs

// ListWallpapers returns every file in <t.Dir>/backgrounds/ that could
// be a wallpaper. Used by TUI wallpaper picker.
func ListWallpapers(t *Theme) ([]string, error) {
	if t == nil {
		return nil, fmt.Errorf("ListWallpapers: nil theme")
	}
	bgDir := filepath.Join(t.Dir, "backgrounds")
	var out []string
	err := fs.WalkDir(os.DirFS(bgDir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".heic" {
			out = append(out, filepath.Join(bgDir, path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}
