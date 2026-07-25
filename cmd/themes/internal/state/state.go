// Package state owns runtime state for themes v4.
//
// State lives at $XDG_STATE_HOME/themes/ (falling back to
// ~/.local/state/themes/) so the repo has zero runtime churn:
//   - state.json  — active theme, per-theme wallpaper, changed_at
//   - current     — symlink to <active-theme-dir>/derived/
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

	// PreviousTheme is what Theme was before the last switch. Enables
	// `themes back` in future revisions (out of P3 scope).
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

// SetCurrent atomically re-points the `current` symlink at the given
// theme's derived/ directory and updates state.json (Theme becomes
// themeName; PreviousTheme becomes the previous Theme).
//
// themeDir is the absolute path to the theme directory (containing
// theme.json and derived/). Callers pass an already-resolved path
// so we don't have to hard-code the themes root here.
//
// Rename-based swap: any reader of `current` observes either the old
// or the new target, never a partial state.
func SetCurrent(themeName, themeDir string) error {
	if themeName == "" || themeDir == "" {
		return fmt.Errorf("state.SetCurrent: theme name and dir required")
	}
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
		updated.Wallpaper = prev.Wallpaper
		if wp, ok := prev.WallpaperByTheme[themeName]; ok {
			updated.Wallpaper = wp
		}
	}
	if err := Save(updated); err != nil {
		return err
	}

	// 2. Atomically swap the `current` symlink via rename(2). Create a
	//    fresh temp symlink then rename over the old one.
	link := currentPath()
	tmp := link + ".tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(derivedPath, tmp); err != nil {
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
