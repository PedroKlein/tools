package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroKlein/tools/cmd/themes/internal/reload"
	"github.com/PedroKlein/tools/cmd/themes/internal/state"
)

// ThemeInfo is the JSON representation of a listed theme.
type ThemeInfo struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Current        bool   `json:"current"`
	HasAlacritty   bool   `json:"has_alacritty"`
	HasPalette     bool   `json:"has_palette"`
	HasNeovim      bool   `json:"has_neovim"`
	WallpaperCount int    `json:"wallpaper_count"`
}

// ListThemes enumerates theme directories under ~/.config/themes/.
// A directory is a theme iff:
//   - name does not begin with "."
//   - directory contains theme.json
//
// Themes are returned sorted alphabetically. The active theme is marked.
func ListThemes() ([]ThemeInfo, error) {
	root := themesRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	activeName := activeState().Theme
	if activeName == "" {
		// Last-resort: read the compat symlink target. On v3 installs this
		// resolves to <themesRoot>/<name>; on v4 fresh installs XDG state is
		// authoritative and this branch is unused.
		if target, err := os.Readlink(currentPath()); err == nil {
			activeName = filepath.Base(target)
			// Guard against the compat pattern `~/.config/themes/.current ->
			// ~/.local/state/themes/current` which would land activeName as
			// literal "current".
			if activeName == "current" || activeName == ".current" {
				activeName = ""
			}
		}
	}
	var themes []ThemeInfo
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		dir := filepath.Join(root, name)
		// Follow symlinks: stow-linked theme dirs report Type()&Symlink,
		// not IsDir(). os.Stat resolves the target.
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		if !fileExists(filepath.Join(dir, "theme.json")) {
			// Not a valid v4 theme dir. Skip silently.
			continue
		}
		t := ThemeInfo{
			Name:         name,
			Path:         dir,
			Current:      name == activeName,
			HasAlacritty: true, // legacy field, always true for v4 themes
			HasPalette:   true, // legacy field, theme.json subsumes palette.toml
			HasNeovim:    fileExists(filepath.Join(dir, "overrides", "nvim.lua")),
		}
		t.WallpaperCount = countWallpapers(filepath.Join(dir, "backgrounds"))
		themes = append(themes, t)
	}
	sort.Slice(themes, func(i, j int) bool { return themes[i].Name < themes[j].Name })
	return themes, nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func countWallpapers(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") ||
			strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".webp") {
			n++
		}
	}
	return n
}

// SetError is returned by Set/SetOptions to signal recoverable failures.
type SetError struct {
	Kind int // ExitError | ExitNotFound
	Msg  string
}

func (e *SetError) Error() string { return e.Msg }

// SetOptions controls a theme switch.
type SetOptions struct {
	// SkipHooks is a list of hook base names to skip (e.g. "opencode", "btop").
	// Passed to hooks/reload-all.sh via THEME_SKIP_HOOKS. Used by live-apply.
	SkipHooks []string
	// Commit records the current-before-switch theme as previous_theme.
	// True for a commit, false for live-apply.
	Commit bool
	// SkipHooksAll when true, does not invoke any hooks. Used by tests.
	SkipHooksAll bool
}

// Set switches the active theme to name. Rules:
//   - Verifies the theme exists.
//   - Atomically swaps ~/.config/themes/.current to point at <name>.
//   - Updates ~/.config/themes/.state.json (theme + wallpaper).
//   - Dispatches hooks/reload-all.sh with THEME_SKIP_HOOKS honored.
//
// The symlink swap is atomic via os.Rename of a temporary link (renameat semantics
// on Unix). On failure to write state, the symlink is rolled back to the prior target.
func Set(name string, opts SetOptions) error {
	dir := themeDir(name)
	if !dirExists(dir) {
		return &SetError{Kind: ExitNotFound, Msg: fmt.Sprintf("theme not installed: %s", name)}
	}
	if !fileExists(filepath.Join(dir, "theme.json")) {
		return &SetError{Kind: ExitError, Msg: fmt.Sprintf("theme dir missing theme.json: %s", dir)}
	}

	// v4 flow (P3.2):
	//   1. derive under lock (produces <dir>/derived/)
	//   2. atomically swap XDG state (state.json + `current` symlink)
	//   3. release lock
	//   4. fire hooks with <dir>/derived/ as argv[1]
	//
	// The lock keeps concurrent `themes set` calls from interleaving the
	// derive+swap phases. Hooks run OUTSIDE the critical section so
	// long-running retint chains never serialize a follow-up swap
	// (Omarchy's pattern).
	unlock, err := acquireSetLock()
	if err != nil {
		return err
	}
	lockReleased := false
	defer func() {
		if !lockReleased {
			unlock()
		}
	}()

	if _, err := deriveThemeV4(dir); err != nil {
		return &SetError{Kind: ExitError, Msg: fmt.Sprintf("derive %s: %v", name, err)}
	}
	if err := state.SetCurrent(name, dir); err != nil {
		return &SetError{Kind: ExitError, Msg: err.Error()}
	}

	unlock()
	lockReleased = true

	if opts.SkipHooksAll {
		return nil
	}
	return runReloadHook(dir, opts.SkipHooks, !opts.Commit)
}

// firstWallpaper returns the first (sorted) wallpaper file under
// <themeDir>/backgrounds/, or "" if none.
func firstWallpaper(themeDir string) string {
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

// swapSymlink atomically points link at target (a name relative to link's dir).
// Uses tmp + rename to avoid ever leaving a dangling link.
func swapSymlink(link, targetRel string) error {
	dir := filepath.Dir(link)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := filepath.Base(link)
	tmp := filepath.Join(dir, base+".swap.tmp")
	// Remove any stale swap file from a prior crash.
	_ = os.Remove(tmp)
	if err := os.Symlink(targetRel, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, link); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// readCurrentTarget returns the target of the .current symlink, or "".
func readCurrentTarget() string {
	target, err := os.Readlink(currentPath())
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return ""
		}
		return ""
	}
	return target
}

// runReloadHook orchestrates every per-app reload hook using the Go
// registry in internal/reload. Replaces the old shell reload-all.sh.
//
// During live-apply scroll (liveApply=true), hook stderr is diverted to
// /tmp/theme-live-apply.log so the picker TUI frame stays clean. On
// commit, errors go to os.Stderr as usual.
func runReloadHook(themeAbsDir string, skip []string, liveApply bool) error {
	return runReloadHookCtx(context.Background(), themeAbsDir, skip, liveApply)
}

// runReloadHookCtx is runReloadHook with a caller-supplied context so the
// TUI settings pane can cancel an in-flight reload when a rapid keypress
// triggers a new one. Zero cost when the caller passes
// context.Background().
//
// NOTE: THEME_LIVE_APPLY is set/unset around reload.RunAll and is racy
// under overlapping in-process reloads. The settings pane serializes
// reloads via cancel-in-flight; callers who spawn overlapping reloads
// must not rely on THEME_LIVE_APPLY toggling. External hooks that read
// this env observe whichever value the last set-caller wrote at exec
// time — acceptable trade-off, documented at plan D3.
func runReloadHookCtx(ctx context.Context, themeAbsDir string, skip []string, liveApply bool) error {
	skipMap := map[string]bool{}
	for _, s := range skip {
		if s != "" {
			skipMap[s] = true
		}
	}
	// Merge THEME_SKIP_HOOKS from the env, even when the caller passed an
	// explicit skip list. This preserves the user's documented escape
	// hatch (e.g. rapid-swap without wallpaper) across all Set entry
	// points — CLI, TUI commit, TUI live-apply, pi extension.
	for name := range reload.SkipList() {
		skipMap[name] = true
	}

	var stderr io.Writer = os.Stderr
	var closer io.Closer
	if liveApply {
		logPath := filepath.Join(os.TempDir(), "theme-live-apply.log")
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			stderr = f
			closer = f
		} else {
			stderr = io.Discard
		}
	}
	if closer != nil {
		defer closer.Close()
	}

	// LIVE_APPLY is a signal external scripts (macos-system.sh) may read.
	// Set it in the process env so exec.Command inherits it.
	if liveApply {
		_ = os.Setenv("THEME_LIVE_APPLY", "1")
		defer os.Unsetenv("THEME_LIVE_APPLY")
	} else {
		_ = os.Unsetenv("THEME_LIVE_APPLY")
	}

	reload.RunAll(ctx, themeAbsDir, reload.Options{
		SkipHooks: skipMap,
		LiveApply: &liveApply,
		Verbose:   os.Getenv("THEME_VERBOSE") != "",
		Stderr:    stderr,
	})
	// RunAll intentionally never returns an error — individual hook
	// failures are collected in Results but do not fail the swap. Callers
	// that need per-hook status should call reload.RunAll directly.
	return nil
}
