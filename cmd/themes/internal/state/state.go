// Package state owns runtime state for themes v4.
//
// State lives at $XDG_STATE_HOME/themes/ (falling back to
// ~/.local/state/themes/) so the repo has zero runtime churn:
//   - state.json  — active theme, per-theme wallpaper, changed_at
//   - current     — symlink to <active-theme-dir>/ (theme root)
//
// The symlink used to point at <theme>/derived/ (see git history for
// the walkback dance that fixed callers). It now points at the theme
// root so callers read `<current>/theme.json` and hooks read
// `<current>/derived/<file>` — one explicit hop instead of a resolve+
// stripbasename tango. Migration: scripts/migrate-themes-flatten.sh.
//
// The main-package v3 State type still exists in cmd/themes/state.go
// for backwards-compat reads on machines mid-migration; this package
// is authoritative for all writes and v4 reads.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// SchemaVersion is the current state.json schema. Bump on any
	// breaking change; older files auto-migrate on next Save.
	SchemaVersion = 1

	stateFilename   = "state.json"
	currentFilename = "current"
)

// State is the shape of $XDG_STATE_HOME/themes/state.json.
type State struct {
	// Theme is the currently active theme name (matches the target of
	// the `current` symlink minus /derived/).
	Theme string `json:"theme"`

	// PreviousTheme is what Theme was before the last switch.
	PreviousTheme string `json:"previous_theme,omitempty"`

	// Wallpaper is the absolute path to the currently applied wallpaper.
	Wallpaper string `json:"wallpaper,omitempty"`

	// WallpaperByTheme remembers each theme's most recent wallpaper so
	// switching themes doesn't forget the user's per-theme picks.
	WallpaperByTheme map[string]string `json:"wallpaper_by_theme,omitempty"`

	// ChangedAt is set to time.Now().UTC().Format(time.RFC3339) on
	// every successful Save. Empty on a fresh install.
	ChangedAt string `json:"changed_at,omitempty"`

	// SchemaVersion — see the package const of the same name.
	SchemaVersion int `json:"schema_version"`
}

// Load reads state.json from XDG_STATE_HOME/themes/. A missing file
// yields a zero-valued State with the current SchemaVersion; not an
// error. Malformed JSON is a hard error.
func Load() (*State, error) {
	return loadFrom(statePath())
}

func loadFrom(path string) (*State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &State{
				WallpaperByTheme: map[string]string{},
				SchemaVersion:    SchemaVersion,
			}, nil
		}
		return nil, fmt.Errorf("state.Load: %w", err)
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("state.Load: parse: %w", err)
	}
	if s.WallpaperByTheme == nil {
		s.WallpaperByTheme = map[string]string{}
	}
	if s.SchemaVersion == 0 {
		s.SchemaVersion = SchemaVersion
	}
	return &s, nil
}

// Save atomically writes state to $XDG_STATE_HOME/themes/state.json.
// Uses write-temp + fsync + rename semantics for durability. Sets
// ChangedAt to now if unset.
func Save(s State) error {
	return saveTo(s, statePath())
}

func saveTo(s State, path string) error {
	if s.WallpaperByTheme == nil {
		s.WallpaperByTheme = map[string]string{}
	}
	if s.SchemaVersion == 0 {
		s.SchemaVersion = SchemaVersion
	}
	if s.ChangedAt == "" {
		s.ChangedAt = time.Now().UTC().Format(time.RFC3339)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("state.Save: mkdir %s: %w", dir, err)
	}
	buf, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("state.Save: marshal: %w", err)
	}
	buf = append(buf, '\n')
	return writeAtomic(path, buf, 0o600)
}

// SwapCurrentSymlink atomically re-points the `current` symlink at
// themeDir WITHOUT touching state.json. Used by the picker's preview
// tier: cursor scrolls swap the visible-surface target so hooks retint
// but state remains what runInteractive started with. Only Commit
// (which calls SetCurrent) mutates state.json.
//
// Precondition: <themeDir>/derived/ must exist. Callers who don't want
// this check can dial down by inlining the os.Symlink + Rename dance.
func SwapCurrentSymlink(themeDir string) error {
	if themeDir == "" {
		return fmt.Errorf("state.SwapCurrentSymlink: theme dir required")
	}
	if _, err := os.Stat(filepath.Join(themeDir, "derived")); err != nil {
		return fmt.Errorf("state.SwapCurrentSymlink: derived dir missing (%s); run 'themes derive' first", themeDir)
	}
	link := currentPath()
	tmp := link + ".tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(themeDir, tmp); err != nil {
		return fmt.Errorf("state.SwapCurrentSymlink: create temp symlink: %w", err)
	}
	if err := os.Rename(tmp, link); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("state.SwapCurrentSymlink: rename symlink: %w", err)
	}
	return nil
}

// SetCurrent atomically re-points the `current` symlink at the given
// theme's root directory and updates state.json (Theme becomes
// themeName; PreviousTheme becomes the previous Theme).
//
// themeDir is the absolute path to the theme directory (containing
// theme.json and derived/). Callers pass an already-resolved path
// so we don't have to hard-code the themes root here.
//
// The symlink target is <themeDir>/ (theme root), NOT <themeDir>/derived/.
// Hooks read derived files as <current>/derived/<file> — one explicit
// hop. See package doc for the migration rationale.
//
// Rename-based swap: any reader of `current` observes either the old
// or the new target, never a partial state.
func SetCurrent(themeName, themeDir string) error {
	if themeName == "" || themeDir == "" {
		return fmt.Errorf("state.SetCurrent: theme name and dir required")
	}
	// Precondition: derived/ must exist. Hooks depend on it — pointing
	// `current` at a theme with no derived files would break the whole
	// swap. This check is invariant, not related to the symlink target.
	derivedPath := filepath.Join(themeDir, "derived")
	if _, err := os.Stat(derivedPath); err != nil {
		return fmt.Errorf("state.SetCurrent: derived dir missing (%s); run 'themes derive' first", derivedPath)
	}

	prev, _ := Load()
	previousTheme := ""
	if prev != nil {
		previousTheme = prev.Theme
	}

	// 1. Update state.json first so a reader that follows the symlink
	//    and then reads state.json sees a consistent view.
	updated := State{
		Theme:            themeName,
		PreviousTheme:    previousTheme,
		WallpaperByTheme: map[string]string{},
	}
	if prev != nil {
		updated.WallpaperByTheme = prev.WallpaperByTheme
		if updated.WallpaperByTheme == nil {
			updated.WallpaperByTheme = map[string]string{}
		}
		// Preserve prev theme's wallpaper choice in the map before we
		// overwrite state.Wallpaper. Otherwise switching A→B→A would
		// forget A's last picked wallpaper.
		if previousTheme != "" && previousTheme != themeName && prev.Wallpaper != "" {
			updated.WallpaperByTheme[previousTheme] = prev.Wallpaper
		}
	}
	// Choose new wallpaper: prefer per-theme memory, else first bg file
	// in the theme's backgrounds/ dir. Never leave a stale wallpaper
	// from the previous theme; the Go wallpaper hook reads state.Wallpaper
	// directly.
	if wp, ok := updated.WallpaperByTheme[themeName]; ok && wp != "" {
		updated.Wallpaper = wp
	} else if bg := firstBackground(themeDir); bg != "" {
		updated.Wallpaper = bg
		updated.WallpaperByTheme[themeName] = bg
	} else {
		// No wallpapers available for this theme; leave empty so the Go
		// wallpaper hook treats it as a no-op rather than reapplying the
		// previous theme's file.
		updated.Wallpaper = ""
	}
	if err := Save(updated); err != nil {
		return err
	}

	// 2. Atomically swap the `current` symlink via rename(2). Create a
	//    fresh temp symlink then rename over the old one. Target is the
	//    theme root; hooks append /derived/<file> as needed.
	link := currentPath()
	tmp := link + ".tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(themeDir, tmp); err != nil {
		return fmt.Errorf("state.SetCurrent: create temp symlink: %w", err)
	}
	if err := os.Rename(tmp, link); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("state.SetCurrent: rename symlink: %w", err)
	}
	return nil
}

// writeAtomic writes data to path via a temp + rename in the same dir.
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// firstBackground returns the first image file (case-insensitive jpg,
// jpeg, png, webp) inside <themeDir>/backgrounds/, sorted
// alphabetically. Returns "" if the dir is missing or has no images.
//
// Kept in the state package because SetCurrent needs to refresh
// state.Wallpaper when a theme swap has no WallpaperByTheme entry;
// otherwise the Go wallpaper hook reads a stale path from the previous theme.
func firstBackground(themeDir string) string {
	bg := filepath.Join(themeDir, "backgrounds")
	entries, err := os.ReadDir(bg)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") ||
			strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".webp") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return filepath.Join(bg, names[0])
}
