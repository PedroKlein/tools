package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "profile":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: pia profile <name> [pi-args...]")
			os.Exit(1)
		}

		launchProfile(os.Args[2], os.Args[3:])
	case "update":
		runUpdate()
	case "sync":
		runSync()
	case "list":
		runList()
	case "show":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: pia show <profile>")
			os.Exit(1)
		}

		runShow(os.Args[2])
	case "doctor":
		runDoctor()
	case "-h", "--help", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nRun 'pia --help' for usage.\n", os.Args[1])
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `pia — Pi profile launcher

Usage:
  pia profile <name> [pi-args...]   Launch pi with the named profile
  pia update                        Update pi and packages for all profiles
  pia sync                          Generate profile directories from definitions
  pia list                          List available profiles
  pia show <profile>                Show profile configuration details
  pia doctor                        Validate all profiles

Profiles are defined in ~/.pi/agent/profiles/<name>/profile.json
Run 'pia sync' after editing profiles to regenerate config directories.
`)
}
