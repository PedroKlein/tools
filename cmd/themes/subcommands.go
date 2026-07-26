package main

import (
	"fmt"
	"os"
	"path/filepath"

	xdgstate "github.com/PedroKlein/tools/cmd/themes/internal/state"
)

// activeState returns the currently-active theme record from the XDG
// state file — single source of truth after the v3 shim was removed in a3.
// Returns a zero-value State (empty Theme) when nothing is recorded,
// never an error — callers decide how to handle empty.
//
// Historically fell back to a v3 flat file at ~/.config/themes/.state.json;
// that path was covered by migrate-themes-v4.sh and migrate-themes-
// flatten.sh. Any install that hasn't run those is unsupported.
func activeState() xdgstate.State {
	if s, err := xdgstate.Load(); err == nil && s != nil {
		return *s
	}
	return xdgstate.State{}
}

// runList — "themes list [--json]".
func runList(_ []string) {
	themes, err := ListThemes()
	if err != nil {
		dieMsg(fmt.Sprintf("list failed: %v", err), ExitError)
	}
	if jsonOutput {
		writeJSON(themes)
		return
	}
	if len(themes) == 0 {
		fmt.Fprintln(os.Stderr, "no themes installed (run: themes install <name>)")
		os.Exit(ExitNotFound)
	}
	for _, t := range themes {
		marker := "  "
		if t.Current {
			marker = "* "
		}
		fmt.Printf("%s%-24s %d wallpapers\n", marker, t.Name, t.WallpaperCount)
	}
}

// runCurrent — "themes current [--json]".
func runCurrent(_ []string) {
	s := activeState()
	if jsonOutput {
		writeJSON(map[string]any{
			"theme":      s.Theme,
			"wallpaper":  s.Wallpaper,
			"changed_at": s.ChangedAt,
		})
		return
	}
	if s.Theme == "" {
		fmt.Fprintln(os.Stderr, "no theme active (run: themes set <name>)")
		os.Exit(ExitNotFound)
	}
	fmt.Println(s.Theme)
	if s.Wallpaper != "" {
		fmt.Fprintf(os.Stderr, "wallpaper: %s\n", s.Wallpaper)
	}
}

// runSet — "themes set <name>".
func runSet(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: themes set <name>")
		os.Exit(ExitError)
	}
	name := args[0]
	err := Set(name, SetOptions{Commit: true})
	if err != nil {
		var se *SetError
		if asErr, ok := err.(*SetError); ok {
			se = asErr
		}
		if se != nil {
			dieMsg(se.Msg, se.Kind)
		}
		dieMsg(err.Error(), ExitError)
	}
	if !jsonOutput {
		fmt.Fprintf(os.Stderr, "themes set: %s\n", name)
	}
}

// runApply — "themes apply" (no swap, just re-derive + re-run hooks).
//
// v4: idempotent + dangling-symlink safe. Reads XDG state; if state.json
// is missing or references a theme that no longer exists, falls back to
// osaka-jade with a warning rather than dying. Always re-derives before
// firing hooks so the derived/ dir is fresh even after a `git checkout`
// on a clone that doesn't have generated files.
func runApply(_ []string) {
	themeName := activeState().Theme

	if themeName != "" && !dirExists(themeDir(themeName)) {
		fmt.Fprintf(os.Stderr, "themes apply: recorded theme %q not installed; falling back to osaka-jade\n", themeName)
		themeName = ""
	}
	if themeName == "" {
		if dirExists(themeDir("osaka-jade")) {
			themeName = "osaka-jade"
			fmt.Fprintln(os.Stderr, "themes apply: no theme recorded; defaulting to osaka-jade")
		} else {
			dieMsg("no theme active and osaka-jade fallback not installed", ExitNotFound)
		}
	}

	dir := themeDir(themeName)
	if !fileExists(filepath.Join(dir, "theme.json")) {
		dieMsg(fmt.Sprintf("theme %s has no theme.json (v3 format); run /theme-import to migrate", themeName), ExitError)
	}

	// Re-derive so derived/ is fresh even on a stale worktree.
	if _, err := deriveThemeV4(dir); err != nil {
		dieMsg(err.Error(), ExitError)
	}

	// Repair the XDG `current` symlink if missing/dangling, OR if it
	// resolves to a different theme than state.json records. Silent
	// drift between state.json.Theme and the symlink was the root cause
	// of 'nothing changes when I switch themes' — hooks reading through
	// .current/* see one theme, state.json says another.
	if xdgstate.CurrentTarget() == "" {
		if err := xdgstate.SetCurrent(themeName, dir); err != nil {
			dieMsg(err.Error(), ExitError)
		}
	} else if xdgstate.IsDrifted() {
		fmt.Fprintf(os.Stderr,
			"themes apply: drift detected — state.Theme=%q but current symlink resolves to %q; re-aligning\n",
			themeName, xdgstate.CurrentTargetTheme())
		if err := xdgstate.SetCurrent(themeName, dir); err != nil {
			dieMsg(err.Error(), ExitError)
		}
	}

	if err := runReloadHook(dir, nil, false); err != nil {
		dieMsg(err.Error(), ExitError)
	}
}

// runWallpaper dispatches wallpaper subsubcommands.
func runWallpaper(args []string) {
	if len(args) == 0 {
		// Interactive picker.
		runWallpaperInteractive()
		return
	}
	switch args[0] {
	case "list":
		s := activeState()
		list, err := WallpaperList(s.Theme)
		if err != nil {
			dieMsg(err.Error(), ExitError)
		}
		if jsonOutput {
			writeJSON(list)
			return
		}
		if len(list) == 0 {
			fmt.Fprintln(os.Stderr, "no wallpapers for", s.Theme)
			os.Exit(ExitNotFound)
		}
		for _, p := range list {
			fmt.Println(p)
		}
	case "set":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: themes wallpaper set <path>")
			os.Exit(ExitError)
		}
		if err := SetWallpaper(args[1]); err != nil {
			dieMsg(err.Error(), ExitError)
		}
	case "random":
		s := activeState()
		pick := RandomWallpaperFor(s.Theme)
		if pick == "" {
			dieMsg("no wallpapers for "+s.Theme, ExitNotFound)
		}
		if err := SetWallpaper(pick); err != nil {
			dieMsg(err.Error(), ExitError)
		}
		if !jsonOutput {
			fmt.Println(pick)
		}
	case "next", "--next", "cycle":
		// Cycle to the wallpaper after the currently-active one. Wraps at
		// the end of the list. Both `next` and `--next` accepted; pi's
		// `/theme cycle-wallpaper` uses `--next`.
		s := activeState()
		if s.Theme == "" {
			dieMsg("no theme active", ExitNotFound)
		}
		if err := CycleWallpaper(s.Theme); err != nil {
			dieMsg(err.Error(), ExitError)
		}
		if !jsonOutput {
			st := activeState()
			if st.Wallpaper != "" {
				fmt.Println(st.Wallpaper)
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown wallpaper subcommand: %s\n", args[0])
		os.Exit(ExitError)
	}
}
