package main

import (
	"fmt"
	"os"
)

// runInstall / runImport are v4 shims. The v3 upstream-file parser has
// been deleted (P1.11); import runs through the Pi slash command
// /theme-import which LLM-generates a validated v4 theme.json.
//
// Both subcommands print a pointer to the slash command and exit non-zero.

func runInstall(args []string) { runImportShim("install", args) }
func runImport(args []string)  { runImportShim("import", args) }

func runImportShim(subcmd string, args []string) {
	url := ""
	if len(args) >= 1 {
		url = args[0]
	}
	fmt.Fprintf(os.Stderr,
		"themes %s: superseded by the Pi slash command /theme-import.\n", subcmd)
	if url != "" {
		fmt.Fprintf(os.Stderr, "  In Pi, run:  /theme-import %s\n", url)
	} else {
		fmt.Fprintln(os.Stderr, "  In Pi, run:  /theme-import <url-or-local-path>")
	}
	fmt.Fprintln(os.Stderr,
		"  See:  docs/plans/theme-switcher-v4.md P4")
	os.Exit(ExitError)
}
