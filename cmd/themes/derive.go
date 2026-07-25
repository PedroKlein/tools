package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// runDerive implements `themes derive [<name>]`.
//
// Regenerates every derived file under <themeDir>/derived/ from
// <themeDir>/theme.json.
//
// Themes without theme.json (unmigrated v3-format bundles) fail with a
// pointer at /theme-import. If <name> is omitted, uses the currently
// active theme.
func runDerive(args []string) {
	var name string
	switch len(args) {
	case 0:
		s, err := LoadState()
		if err != nil || s.Theme == "" {
			dieMsg("no active theme; usage: themes derive <name>", ExitError)
		}
		name = s.Theme
	case 1:
		name = args[0]
	default:
		dieMsg("usage: themes derive [<name>]", ExitError)
	}

	dir := themeDir(name)
	if !dirExists(dir) {
		dieMsg(fmt.Sprintf("theme not installed: %s", name), ExitNotFound)
	}
	if !fileExists(filepath.Join(dir, "theme.json")) {
		dieMsg(fmt.Sprintf(
			"theme %s has no theme.json (v3 format); run /theme-import to migrate", name),
			ExitError)
	}

	written, err := deriveThemeV4(dir)
	if err != nil {
		dieMsg(err.Error(), ExitError)
	}
	if !jsonOutput {
		fmt.Fprintf(os.Stderr, "themes derive: %s regenerated (%d files → derived/)\n", name, len(written))
	}
}
