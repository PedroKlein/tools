package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func joinNonFlagArgs(args []string) string {
	var parts []string

	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}

		if strings.HasPrefix(a, "-") {
			skipNext = true // skip flag and its value
			continue
		}

		parts = append(parts, a)
	}

	return strings.Join(parts, " ")
}

func run(args []string) int {
	parsed := ParseRunArgs(args)

	// Determine repo ID
	repoID := parsed.Repo
	if repoID == "" {
		switch {
		case parsed.Global:
			repoID = GlobalRepoID
		case parsed.Work:
			repoID = WorkRepoID
		default:
			repoID = DetectRepoID()
		}
	}

	// Initialize store
	store := NewStore(DefaultRoot())

	switch parsed.Command {
	case "tui":
		if err := Run(store, repoID); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			return 1
		}

		return 0
	case "add":
		if !parsed.Quick {
			// Open TUI form with AI pre-fill for review
			text := joinNonFlagArgs(parsed.Args)
			if err := RunAdd(store, repoID, text); err != nil {
				fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
				return 1
			}

			return 0
		}

		cli := NewCLI(store, repoID)

		return cli.Run("add", parsed.Args, parsed.JSON)
	case "-h", "--help", "help":
		cli := NewCLI(store, repoID)
		return cli.Run("help", nil, false)
	default:
		cli := NewCLI(store, repoID)
		return cli.Run(parsed.Command, parsed.Args, parsed.JSON)
	}
}
