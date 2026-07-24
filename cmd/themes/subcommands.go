package main

import (
	"fmt"
	"os"
)

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
	s, err := LoadState()
	if err != nil {
		dieMsg(fmt.Sprintf("state load failed: %v", err), ExitError)
	}
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

// runApply — "themes apply" (no swap, just re-run hooks).
func runApply(_ []string) {
	s, err := LoadState()
	if err != nil {
		dieMsg(fmt.Sprintf("state load failed: %v", err), ExitError)
	}
	if s.Theme == "" {
		dieMsg("no theme active", ExitNotFound)
	}
	if err := runReloadHook(themeDir(s.Theme), nil, false); err != nil {
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
		s, err := LoadState()
		if err != nil {
			dieMsg(err.Error(), ExitError)
		}
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
		s, err := LoadState()
		if err != nil {
			dieMsg(err.Error(), ExitError)
		}
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
		s, err := LoadState()
		if err != nil {
			dieMsg(err.Error(), ExitError)
		}
		if s.Theme == "" {
			dieMsg("no theme active", ExitNotFound)
		}
		if err := CycleWallpaper(s.Theme); err != nil {
			dieMsg(err.Error(), ExitError)
		}
		if !jsonOutput {
			if st, err := LoadState(); err == nil {
				fmt.Println(st.Wallpaper)
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown wallpaper subcommand: %s\n", args[0])
		os.Exit(ExitError)
	}
}
