package main

import (
	"fmt"
	"os"
)

// runInstall is now a shim. The v3 upstream-file parser (colors.toml ->
// alacritty.toml + palette.toml conversion, git clone + copy) has been
// deleted; theme import runs through the Pi slash command /theme-import
// instead, which LLM-generates a validated v4 theme.json.
//
// P4 replaces this shim with a proper "run /theme-import <url>"
// instruction. For P1.11 the shim exits with a helpful pointer.
func runInstall(args []string) {
	url := ""
	if len(args) >= 1 {
		url = args[0]
	}
	_ = url
	fmt.Fprintln(os.Stderr,
		"themes install: superseded by the Pi slash command /theme-import.")
	fmt.Fprintln(os.Stderr,
		"  In Pi, run:  /theme-import <url-or-local-path>")
	fmt.Fprintln(os.Stderr,
		"  See:  docs/plans/theme-switcher-v4.md P4")
	os.Exit(ExitError)
}

// runInstallDerive is a stub kept only so callers referencing it still
// compile. P4 removes the callsite entirely.
func runInstallDerive(themeAbsDir string) error {
	_ = themeAbsDir
	return fmt.Errorf("themes install has been superseded; run /theme-import in Pi")
}
