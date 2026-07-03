package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// OpenResult is the JSON output for repos open.
type OpenResult struct {
	Session string `json:"session"`
	Path    string `json:"path"`
	Created bool   `json:"created"` // true = new session, false = switched to existing
}

func runOpen(args []string) {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	repoPath, relPath, err := resolveRepo(query)
	if err != nil {
		if jsonOutput {
			writeJSONError(err.Error(), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(ExitError)
	}

	// Check tmux is available
	if _, err := exec.LookPath("tmux"); err != nil {
		if jsonOutput {
			writeJSONError("tmux not found in PATH", ExitError)
		}

		fmt.Fprintln(os.Stderr, "error: tmux not found in PATH")
		os.Exit(ExitError)
	}

	// Session name: repo name (last component), dots replaced with dashes
	sessionName := filepath.Base(relPath)
	sessionName = strings.ReplaceAll(sessionName, ".", "-")

	worktreeDir := defaultWorktreePath(repoPath)

	// Check if session already exists
	sessionExists := exec.Command("tmux", "has-session", "-t", sessionName).Run() == nil
	created := !sessionExists

	if !sessionExists {
		// Create new detached session with nvim . in the worktree dir
		cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", worktreeDir, "nvim", ".")
		if out, err := cmd.CombinedOutput(); err != nil {
			if jsonOutput {
				writeJSONError(fmt.Sprintf("failed to create tmux session: %v", err), ExitError)
			}

			fmt.Fprintf(os.Stderr, "error creating tmux session: %v\n%s", err, out)
			os.Exit(ExitError)
		}
	}

	// Emit JSON before switching (after switch we may lose the terminal)
	if jsonOutput {
		result := OpenResult{Session: sessionName, Path: worktreeDir, Created: created}
		data, _ := json.Marshal(result)
		fmt.Println(string(data))
	}

	// Switch or attach
	inTmux := os.Getenv("TMUX") != ""
	if inTmux {
		_ = exec.Command("tmux", "switch-client", "-t", sessionName).Run()

		if !jsonOutput {
			// switch-client is instant; message is visible briefly before the switch
			action := "opened"
			if !created {
				action = "switched to"
			}

			fmt.Printf("%s %s (session: %s)\n", action, relPath, sessionName)
		}
	} else {
		// Attach blocks until the user detaches — no message needed
		cmd := exec.Command("tmux", "attach-session", "-t", sessionName)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run() //nolint:errcheck,gosec // attach blocks until user detaches; non-zero exit is normal
	}
}
