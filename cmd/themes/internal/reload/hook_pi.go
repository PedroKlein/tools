package reload

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// piProfileDirs is the list of pi profile directories the hook writes
// into. Missing dirs are silently skipped (not every user has all four).
//
// Var (not const) so tests can inject a fixture path.
var piProfileDirs = func() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return []string{
		filepath.Join(home, ".pi", "agent"),
		filepath.Join(home, ".pi", "agent-quick"),
		filepath.Join(home, ".pi", "agent-research"),
		filepath.Join(home, ".pi", "agent-personal"),
	}
}

// hookPi ports .hooks/pi.sh to Go. Reads <themeDir>/derived/pi.json,
// forces its `name` field to "current", and atomically writes the result
// into each existing pi profile's themes/current.json. Pi's built-in
// file watcher fires onThemeChange on the byte-overwrite and retints
// the UI in place — no /reload, no session interruption.
//
// Preview + Commit: RunPreview=true, RunCommit=true. Fast (<10 ms
// typical) because payload is one JSON file ≤4 KB × 4 profile dirs.
//
// Atomic write: temp file in the same dir + rename(2). Preserves the
// invariant that pi never observes a half-written current.json.
func hookPi(_ context.Context, themeDir string) error {
	src := filepath.Join(themeDir, "derived", "pi.json")
	if _, err := os.Stat(src); err != nil {
		// Fallback for pre-flatten installs (one release deprecation).
		alt := filepath.Join(themeDir, "pi.json")
		if _, err2 := os.Stat(alt); err2 != nil {
			return nil // no pi payload; silent no-op
		}
		src = alt
	}

	raw, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("hookPi: read %s: %w", src, err)
	}
	// Parse + rewrite .name = "current".
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("hookPi: parse %s: %w", src, err)
	}
	doc["name"] = "current"
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("hookPi: marshal: %w", err)
	}
	// Preserve the trailing newline convention.
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}

	dirs := piProfileDirs()
	var writeErr error
	for _, prof := range dirs {
		if _, err := os.Stat(prof); err != nil {
			continue // profile not installed
		}
		themesDir := filepath.Join(prof, "themes")
		if err := os.MkdirAll(themesDir, 0o755); err != nil {
			writeErr = fmt.Errorf("hookPi: mkdir %s: %w", themesDir, err)
			continue
		}
		dst := filepath.Join(themesDir, "current.json")
		if err := writeAtomic(dst, out); err != nil {
			writeErr = fmt.Errorf("hookPi: write %s: %w", dst, err)
			continue
		}
	}
	return writeErr
}

// writeAtomic writes `content` to `dst` via a same-directory temp file +
// rename. On macOS this triggers a single FSEvents CHANGE for the final
// path, which pi's file watcher observes. Never writes into the temp
// directory of a different filesystem — rename would be non-atomic.
//
// Failure modes: mkdir/temp/write/close/rename errors bubble up. On
// success, the temp file no longer exists.
func writeAtomic(dst string, content []byte) error {
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, filepath.Base(dst)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // best-effort on any error path
	if _, err := tmp.Write(content); err != nil {
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
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}
