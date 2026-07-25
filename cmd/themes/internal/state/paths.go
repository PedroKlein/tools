package state

import (
	"os"
	"path/filepath"
)

// Root returns the absolute path to $XDG_STATE_HOME/themes/. Honors
// XDG_STATE_HOME when set (with a themes suffix) and otherwise falls
// back to ~/.local/state/themes/.
//
// Never returns an empty string; if $HOME is unset (edge case in CI /
// sandbox), returns "/tmp/themes-state" as a last resort so the CLI
// can still emit a diagnostic without panicking.
func Root() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "themes")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		if h := os.Getenv("HOME"); h != "" {
			home = h
		} else {
			return "/tmp/themes-state"
		}
	}
	return filepath.Join(home, ".local", "state", "themes")
}

// statePath returns the absolute path to state.json.
func statePath() string {
	return filepath.Join(Root(), stateFilename)
}

// currentPath returns the absolute path to the `current` symlink.
func currentPath() string {
	return filepath.Join(Root(), currentFilename)
}

// StatePath is the exported form for callers that need to read/write
// state.json directly (hooks, migration scripts). Prefer Load/Save.
func StatePath() string { return statePath() }

// CurrentPath is the exported form for callers that need the symlink
// path (hooks, setup scripts). Prefer SetCurrent().
func CurrentPath() string { return currentPath() }

// CurrentTarget returns the absolute path the `current` symlink
// resolves to, or "" if the symlink is missing or dangling.
func CurrentTarget() string {
	target, err := os.Readlink(currentPath())
	if err != nil {
		return ""
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(currentPath()), target)
	}
	if _, err := os.Stat(target); err != nil {
		return ""
	}
	return target
}
