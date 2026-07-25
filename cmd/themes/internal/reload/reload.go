// Package reload orchestrates per-app theme reload after a theme swap.
//
// Two hook categories:
//
//  1. Inline hooks (declared in this package as Go code): fast operations
//     that don't need a subprocess. Signals, direct commands, JSON edits.
//     Consolidated here to eliminate the .sh dust bunny of one-liner
//     hooks that used to live under configs-shared/.config/themes/.hooks/.
//
//  2. External hooks (.sh files): non-trivial logic that benefits from
//     shell idioms (bash arrays, tmux commands, OSC broadcast bit-fiddling,
//     complex fallback chains). Discovered by scanning the .hooks/ dir.
//
// Callers invoke RunAll(themeDir, opts) which runs every enabled hook in
// parallel via goroutines. Per-hook errors are logged but never fatal to
// the caller — a broken bat cache shouldn't block the rest of the theme
// swap.
package reload

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// Kind is the hook-execution strategy.
type Kind int

const (
	// KindNoop declares the hook has no runtime action; theme applies on
	// next app launch. Kept in the registry for documentation +
	// per-app opt-out logic.
	KindNoop Kind = iota

	// KindCommand runs a shell command via exec (no shell interpretation).
	// Cmd + Args are the exact argv. Slow if the target binary is slow;
	// consider Signal or Inline for hot paths.
	KindCommand

	// KindSignal sends a signal to every running process matching Target.
	// Uses os.FindProcess + Signal; no pkill fork. Silently no-ops when
	// no matching process is found.
	KindSignal

	// KindInline calls a Go function with the theme dir. Reserved for
	// operations that can be reduced to a few stdlib calls (e.g.
	// opencode's tui.json .theme field edit).
	KindInline

	// KindExternal invokes an .sh file under the hooks dir. Used for
	// non-trivial logic (osc-broadcast, macos-system, wallpaper, pi,
	// tmux). The .sh file receives the theme dir as its first arg.
	KindExternal
)

// Hook is one entry in the reload registry.
type Hook struct {
	// Name is a short unique label. Used for THEME_SKIP_HOOKS matching
	// and verbose logging.
	Name string

	// Kind selects the runtime strategy.
	Kind Kind

	// LiveApply controls whether the hook fires during picker scroll
	// preview (THEME_LIVE_APPLY=1). false means commit-only.
	LiveApply bool

	// Timeout is the maximum time RunAll waits for this hook. Default 4s
	// when zero.
	Timeout time.Duration

	// --- Kind-specific fields ---

	// Cmd is the command executable for KindCommand.
	Cmd string

	// Args are the arguments passed to Cmd. Never shell-interpreted.
	Args []string

	// Signal is the signal name (e.g. "SIGUSR1") for KindSignal.
	Signal string

	// SignalTarget is the process name for KindSignal (e.g. "nvim").
	// Matched via os.Process discovery (see hooks.go).
	SignalTarget string

	// Fn is the Go function invoked for KindInline. Receives the theme
	// directory and a cancellation context. Implementations MUST honor
	// ctx.Done() by returning promptly — Go has no forcible goroutine
	// cancel, so a Fn that ignores ctx will leak past its per-hook
	// timeout. All in-tree inline hooks are file/mem ops that return in
	// milliseconds, so this is a soft contract; audit new inline hooks
	// carefully.
	Fn func(ctx context.Context, themeDir string) error

	// Script is the basename of the .sh file for KindExternal (relative
	// to the hooks dir, e.g. "osc-broadcast.sh").
	Script string
}

// Registry returns the canonical hook list in a stable order. Callers may
// filter via SkipHooks / LiveApply / os-specific gating (e.g. sketchybar
// on Darwin only).
//
// New hooks: add to hooks.go. This function stays alphabetical by Name
// for readability.
func Registry() []Hook {
	return append([]Hook{}, registry...)
}

// SkipList parses a THEME_SKIP_HOOKS env var into a set of names to omit.
// Format: comma-separated names ("bat,sketchybar"). Empty string returns
// an empty set.
func SkipList() map[string]bool {
	env := os.Getenv("THEME_SKIP_HOOKS")
	if env == "" {
		return nil
	}
	out := map[string]bool{}
	start := 0
	for i := 0; i <= len(env); i++ {
		if i == len(env) || env[i] == ',' {
			name := env[start:i]
			if name != "" {
				out[name] = true
			}
			start = i + 1
		}
	}
	return out
}

// LiveApplyMode reports whether the caller is a picker scroll preview
// (THEME_LIVE_APPLY=1). Hooks with LiveApply=false are skipped in this mode.
func LiveApplyMode() bool {
	return os.Getenv("THEME_LIVE_APPLY") == "1"
}

// FilterHooks returns the subset of the registry that should fire for
// this invocation. Applies skip list, live-apply mode, and OS gating.
func FilterHooks(skip map[string]bool, liveApply bool) []Hook {
	out := make([]Hook, 0, len(registry))
	for _, h := range registry {
		if skip[h.Name] {
			continue
		}
		if liveApply && !h.LiveApply {
			continue
		}
		if !osMatches(h) {
			continue
		}
		out = append(out, h)
	}
	return out
}

// osMatches applies per-hook OS filters. Currently:
//   - sketchybar: Darwin only
//   - macos-system: Darwin only
//   - anything else: cross-platform
func osMatches(h Hook) bool {
	// Central OS gate keeps hooks.go free of runtime.GOOS checks. Extend
	// this table when new platform-specific hooks are added.
	darwinOnly := map[string]bool{
		"sketchybar":   true,
		"macos-system": true,
	}
	if darwinOnly[h.Name] {
		return runtimeGOOS() == "darwin"
	}
	return true
}

// runtimeGOOS is a var so tests can override. Never mutated at runtime;
// re-exported as a func so callers get compile-time linkage rather than
// a runtime.GOOS shadow.
var runtimeGOOS = func() string { return goos }

// hookScript resolves an external hook's absolute path. Defaults to
// ~/.config/themes/.hooks/<name>.
func hookScript(name string) string {
	base := hooksDir()
	return filepath.Join(base, name)
}

// hooksDir returns <themes-root>/.hooks. Honors XDG_CONFIG_HOME (so
// tests and XDG installs work) and falls back to ~/.config/themes/.hooks
// otherwise. Callers should treat the path as best-effort: it may not
// exist on a fresh install; nil-safe reads and stat-then-run are the
// idiom.
func hooksDir() string {
	return filepath.Join(themesRoot(), ".hooks")
}

// themesRoot mirrors the outer package's themesRoot(). Duplicated to keep
// internal/reload dep-free; both must honor XDG_CONFIG_HOME identically
// or the stale-reload guard mis-fires under XDG installs.
//
// Fallback semantics match cmd/themes/paths.go: os.UserHomeDir() first,
// then $HOME env, then "" (which will produce an obviously-wrong path so
// callers can detect it via stat).
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
