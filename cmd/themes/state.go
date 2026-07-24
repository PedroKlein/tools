package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// State is the shape of ~/.config/themes/.state.json.
//
// Contract:
//   - Theme is the currently-committed theme name (the theme .current points at).
//   - Wallpaper is the currently-set wallpaper path, or empty.
//   - WallpaperByTheme remembers each theme's last wallpaper across switches.
//   - ChangedAt is set on every commit (empty on a fresh install).
//   - SchemaVersion allows forward migrations.
type State struct {
	Theme            string            `json:"theme"`
	Wallpaper        string            `json:"wallpaper"`
	WallpaperByTheme map[string]string `json:"wallpaper_by_theme"`
	ChangedAt        string            `json:"changed_at"`
	SchemaVersion    int               `json:"schema_version"`
}

const currentSchemaVersion = 1

// LoadState reads state.json from ~/.config/themes/.state.json.
// A missing file yields a zero-valued State with SchemaVersion set; not an error.
func LoadState() (*State, error) {
	return loadStateFrom(statePath())
}

func loadStateFrom(path string) (*State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &State{
				WallpaperByTheme: map[string]string{},
				SchemaVersion:    currentSchemaVersion,
			}, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if s.WallpaperByTheme == nil {
		s.WallpaperByTheme = map[string]string{}
	}
	if s.SchemaVersion == 0 {
		s.SchemaVersion = currentSchemaVersion
	}
	return &s, nil
}

// SaveState atomically writes state to ~/.config/themes/.state.json.
// Uses write-temp + fsync + rename for durability.
func (s *State) Save() error {
	return s.saveTo(statePath())
}

func (s *State) saveTo(path string) error {
	if s.WallpaperByTheme == nil {
		s.WallpaperByTheme = map[string]string{}
	}
	if s.SchemaVersion == 0 {
		s.SchemaVersion = currentSchemaVersion
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	tmp, err := os.CreateTemp(dir, ".state.json.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Best-effort cleanup on any error path below.
	defer os.Remove(tmpName)
	if _, err := tmp.Write(buf); err != nil {
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
	return os.Rename(tmpName, path)
}

// nowUTC returns an RFC3339 timestamp in UTC. Overridable for tests.
var nowUTC = func() string {
	return time.Now().UTC().Format(time.RFC3339)
}
