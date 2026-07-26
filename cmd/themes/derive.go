package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// runDerive implements `themes derive [<name>] [--force]`.
//
// Regenerates every derived file under <themeDir>/derived/ from
// <themeDir>/theme.json. Skips the emit pipeline when a matching
// .stamp exists AND every emitter output is present; use --force to
// bypass the cache.
//
// Themes without theme.json (unmigrated v3-format bundles) fail with a
// pointer at /theme-import. If <name> is omitted, uses the currently
// active theme.
func runDerive(args []string) {
	force := false
	var positional []string
	for _, a := range args {
		switch a {
		case "--force", "-f":
			force = true
		default:
			positional = append(positional, a)
		}
	}
	var name string
	switch len(positional) {
	case 0:
		s := activeState()
		if s.Theme == "" {
			dieMsg("no active theme; usage: themes derive <name> [--force]", ExitError)
		}
		name = s.Theme
	case 1:
		name = positional[0]
	default:
		dieMsg("usage: themes derive [<name>] [--force]", ExitError)
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

	written, err := deriveThemeV4WithForce(dir, force)
	if err != nil {
		dieMsg(err.Error(), ExitError)
	}
	if !jsonOutput {
		if len(written) == 0 {
			fmt.Fprintf(os.Stderr, "themes derive: %s no changes, skipping (stamp match)\n", name)
		} else {
			fmt.Fprintf(os.Stderr, "themes derive: %s regenerated (%d files → derived/)\n", name, len(written))
		}
	}
}
