package reload

import (
	"os"
	"path/filepath"
)

// isStaleReload reports whether a reload for themeDir is superseded by a
// newer concurrent theme swap.
//
// Compares the basename of themeDir against the resolved target of
// ~/.config/themes/.current. If they differ, another Set() call already
// swapped .current to a different theme after we released the flock, and
// its own RunAll will handle the retint. Firing this hook chain would
// overwrite pi.json / ghostty.conf / etc. with stale colors.
//
// Returns false when we can't read .current (fresh install, missing
// symlink) so the reload proceeds — we only bail on a demonstrated
// mismatch.
func isStaleReload(themeDir string) bool {
	current := filepath.Join(themesRoot(), ".current")
	target, err := os.Readlink(current)
	if err != nil {
		return false
	}
	want := filepath.Base(themeDir)
	// target may be a relative or absolute path; compare basenames.
	return filepath.Base(target) != want
}
