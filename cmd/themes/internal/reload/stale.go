package reload

import (
	"os"
	"path/filepath"
)

// isStaleReload reports whether a reload for themeDir is superseded by a
// newer concurrent theme swap.
//
// Reads ~/.config/themes/.current and extracts the active theme name.
// If it names a different theme than the one this reload is for, another
// Set() call already swapped after we released the flock; its own
// RunAll will handle the retint. Firing this hook chain would overwrite
// pi.json / ghostty.conf / etc. with stale colors.
//
// v4 layout the resolver must walk:
//
//	~/.config/themes/.current -> ~/.local/state/themes/current
//	                          -> ~/.config/themes/<name>/derived
//
// v3 layout the resolver must still handle:
//
//	~/.config/themes/.current -> <name>              (relative)
//	~/.config/themes/.current -> /abs/path/<name>    (absolute)
//
// Strategy: try filepath.EvalSymlinks first (handles the full v4 chain
// and resolves stow-linked repo paths). If that fails — the target
// doesn't exist yet, permissions, etc. — fall back to a single-hop
// os.Readlink so we still trip on "newer swap pointed .current at a
// theme dir that hasn't been derived yet" cases.
//
// Returns false when neither strategy resolves — fresh install, missing
// symlink — so the reload proceeds. We only bail on a demonstrated
// mismatch.
func isStaleReload(themeDir string) bool {
	current := filepath.Join(themesRoot(), ".current")
	want := themeName(themeDir)

	if resolved, err := filepath.EvalSymlinks(current); err == nil {
		return themeName(resolved) != want
	}
	if target, err := os.Readlink(current); err == nil {
		return themeName(target) != want
	}
	return false
}

// themeName extracts the theme name from a path that may be a bare theme
// name, a theme directory, or a theme's derived/ subdirectory.
func themeName(p string) string {
	if filepath.Base(p) == "derived" {
		p = filepath.Dir(p)
	}
	return filepath.Base(p)
}
