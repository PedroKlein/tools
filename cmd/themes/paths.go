package main

import (
	"os"
	"path/filepath"
)

// Layout constants. All paths inside ~/.config/themes/ are prefixed with a
// dot when they carry switcher metadata; theme directories themselves are
// bare names at the top level. State lives at $XDG_STATE_HOME/themes/
// (see internal/state) — not at ~/.config/themes/.state.json anymore.
const (
	currentLink = ".current"
	hooksDir    = ".hooks"
	binDir      = ".bin"
)

// themesRoot returns the absolute path to ~/.config/themes/.
// Honors XDG_CONFIG_HOME when set.
func themesRoot() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "themes")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config", "themes")
}

// currentPath returns the absolute path to the .current symlink.
func currentPath() string {
	return filepath.Join(themesRoot(), currentLink)
}

// CurrentThemeDir returns the absolute path to the active theme directory
// (resolving the .current symlink). Falls back to themesRoot() if the
// symlink is missing or broken. Exported for use from tui.go.
//
// Post-flatten layout: `.current` -> XDG `current` -> <theme>/. Callers
// read `<current>/theme.json` directly; hooks read `<current>/derived/`.
//
// Compat: if we encounter a stale pre-flatten install where the chain
// still lands on `<theme>/derived`, walk up one level. Kept for one
// release so users mid-migration don't hit a broken TUI. Remove after
// the deprecation window closes.
func CurrentThemeDir() string {
	p := currentPath()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p // caller can still stat and get an error; better than ""
	}
	if filepath.Base(resolved) == "derived" {
		return filepath.Dir(resolved)
	}
	return resolved
}

// themeDir returns the absolute path to a theme directory by name.
func themeDir(name string) string {
	return filepath.Join(themesRoot(), name)
}

// hookScript returns the absolute path to a per-app hook script.
func hookScript(name string) string {
	return filepath.Join(themesRoot(), hooksDir, name)
}

// writeFileAtomic writes data to path via a temp file in the same directory,
// fsyncs the file, renames it, then fsyncs the parent directory. Prevents
// crash/kill/disk-full from truncating the destination and (via parent
// fsync) ensures the rename is durable across a power loss.
//
// Preserves existing file permissions if `path` already exists; otherwise
// applies `mode` (typical: 0o644). This lets users chmod theme files
// without them being reset on the next `themes derive`.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	finalMode := mode
	if st, err := os.Stat(path); err == nil {
		finalMode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // best-effort cleanup on error
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, finalMode); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	// Fsync the parent directory so the rename is durable across power
	// loss. Ignored on filesystems that don't support it; best-effort.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}
