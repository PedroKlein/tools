package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// acquireSetLock takes an exclusive flock on a well-known path so
// concurrent `themes set` invocations serialize instead of interleaving.
// Returns a release closure that both unlocks and closes the fd.
//
// Rationale: the TUI's live-apply fires Set() on every scroll after a
// 150ms debounce. Fast cursor movement or two shells running `themes set`
// in parallel would otherwise race on .current symlink swap + state
// write. flock is the simplest POSIX-portable coordination primitive
// and blocks only within this narrow critical section (~500ms).
//
// Not fatal on flock failure: we log and continue lock-less rather than
// prevent the swap outright. Themes are user data; getting locked out
// of your own dotfiles is a bad UX.
func acquireSetLock() (func(), error) {
	lockDir := os.TempDir()
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		lockDir = xdg
	}
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return func() {}, nil // fall through lockless
	}
	path := filepath.Join(lockDir, "theme-set.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return func() {}, nil // fall through lockless
	}
	// Exclusive, blocking flock. Blocks a peer trying to grab the same
	// lock until this process finishes and releases (via defer). Held
	// only for the duration of Set() — hook subprocesses spawned by
	// runReloadHook() run within the lock, which serializes writes to
	// per-app config files (~/.pi/*/themes/current.json, etc.).
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return func() {}, nil // fall through lockless
	}
	release := func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}
	return release, nil
}
