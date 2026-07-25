package main

import (
	"fmt"
	"os"
)

// jsonOutput is declared in json.go.

//nolint:gocyclo // CLI dispatcher with one case per sub-command
func main() {
	jsonOutput, args := parseGlobalFlags(os.Args)
	_ = jsonOutput // set into the package-level via re-assignment below
	setJSONOutput(jsonOutput)

	if len(args) < 2 {
		runTUI(nil)
		return
	}

	cmd := args[1]
	cmdArgs := args[2:]

	switch cmd {
	case "list":
		runList(cmdArgs)
	case "current":
		runCurrent(cmdArgs)
	case "set":
		runSet(cmdArgs)
	case "apply":
		runApply(cmdArgs)
	case "wallpaper":
		runWallpaper(cmdArgs)
	case "install":
		runInstall(cmdArgs)
	case "import":
		runImport(cmdArgs)
	case "derive":
		runDerive(cmdArgs)
	case "validate":
		runValidate(cmdArgs)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\nrun 'themes --help' for usage.\n", cmd)
		os.Exit(ExitError)
	}
}

func setJSONOutput(v bool) { jsonOutput = v }

// parseGlobalFlags extracts --json from anywhere in args and returns the
// cleaned args slice without it.
func parseGlobalFlags(args []string) (jsonFlag bool, cleaned []string) {
	for _, arg := range args {
		if arg == "--json" {
			jsonFlag = true
		} else {
			cleaned = append(cleaned, arg)
		}
	}
	return jsonFlag, cleaned
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `themes — cross-app switcher across 15 apps

Usage:
  themes                        Open interactive picker (default)
  themes list [--json]          List installed themes
  themes current [--json]       Print active themes state and wallpaper
  themes set <name>             Apply the given themes bundle (headless)
  themes apply                  Re-run hooks against the active themes (no swap)
  themes wallpaper              Interactive wallpaper picker (grid preview)
  themes wallpaper set <path>   Set wallpaper directly
  themes wallpaper next         Cycle to next wallpaper for active themes
  themes wallpaper random       Random wallpaper from the active themes
  themes install <url|name>     Import an Omarchy-marketplace bundle
  themes import <url|path>      (same as install; both are v4 shims for /theme-import)
  themes derive <name>          Regenerate the 14 derived files (13 apps + .macos.json)
  themes validate <name|path>   Validate theme.json against the v1 schema

Global Flags:
  --json    JSON output on read commands (list, current)

Exit Codes:
  0  Success
  1  General error
  2  Ambiguous input
  3  Themes bundle not installed / manifest missing`)
}
