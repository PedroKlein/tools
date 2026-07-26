package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

// stampFilename is the derive-cache marker file inside <theme>/derived/.
// Contains the hex-encoded sha256 of the theme's semantic inputs
// (theme.json + every file under overrides/, if present). Presence of a
// matching stamp AND every emitter output file lets `themes derive` and
// `themes set` skip the emit pipeline.
const stampFilename = ".stamp"

// computeThemeStamp reads <theme>/theme.json and (if present) every file
// under <theme>/overrides/ and returns the sha256 of their concatenated
// contents (with a null-byte separator between files for unambiguity).
//
// Errors bubble up untouched so callers can distinguish "no theme.json"
// (structural bug) from "cache miss".
func computeThemeStamp(themeDir string) (string, error) {
	h := sha256.New()

	// 1. theme.json — the primary input.
	tj, err := os.ReadFile(filepath.Join(themeDir, "theme.json"))
	if err != nil {
		return "", err
	}
	_, _ = h.Write(tj)
	_, _ = h.Write([]byte{0})

	// 2. overrides/** — sorted by relative path for determinism.
	overridesRoot := filepath.Join(themeDir, "overrides")
	var overridePaths []string
	err = filepath.WalkDir(overridesRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		overridePaths = append(overridePaths, p)
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	sort.Strings(overridePaths)
	for _, p := range overridePaths {
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return "", rerr
		}
		// Hash path + content so renames are noticed too.
		rel, _ := filepath.Rel(themeDir, p)
		_, _ = h.Write([]byte(rel))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(b)
		_, _ = h.Write([]byte{0})
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// readStamp returns the previously-persisted stamp inside
// <theme>/derived/.stamp, or "" if the file is missing/unreadable.
// A truncated or unreadable stamp is treated as a cache miss, never
// an error — an interrupted derive shouldn't wedge future runs.
func readStamp(themeDir string) string {
	b, err := os.ReadFile(filepath.Join(themeDir, "derived", stampFilename))
	if err != nil {
		return ""
	}
	// Trim optional trailing newline for shell-friendly hand-edits.
	s := string(b)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// writeStamp persists the stamp atomically inside <theme>/derived/.
func writeStamp(themeDir, stamp string) error {
	dir := filepath.Join(themeDir, "derived")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, stampFilename), []byte(stamp+"\n"), 0o644)
}

// allEmitterOutputsPresent verifies every emitter's output file exists
// inside <themeDir>/derived/. Cache-validity depends on this AND a
// matching stamp — either alone is insufficient.
func allEmitterOutputsPresent(themeDir string) bool {
	derived := filepath.Join(themeDir, "derived")
	for _, e := range palette.EmittersV4 {
		if _, err := os.Stat(filepath.Join(derived, e.Filename())); err != nil {
			return false
		}
	}
	return true
}

// deriveCacheHits returns (stamp, true) when the derive cache is
// valid — same theme content AND every emitter output exists. Returns
// ("", false) on any cache miss. Never errors: any hash/IO failure is
// treated as a cache miss so the caller falls through to a full derive.
func deriveCacheHits(themeDir string) (stamp string, hit bool) {
	stamp, err := computeThemeStamp(themeDir)
	if err != nil {
		return "", false
	}
	prev := readStamp(themeDir)
	if prev == "" || prev != stamp {
		return stamp, false
	}
	if !allEmitterOutputsPresent(themeDir) {
		return stamp, false
	}
	return stamp, true
}
