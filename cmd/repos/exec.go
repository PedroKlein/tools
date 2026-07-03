package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExecResult is the JSON representation of a single repo's exec outcome.
type ExecResult struct {
	Repo     string `json:"repo"`
	Path     string `json:"path"`
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

//nolint:gocognit,gocyclo // complex CLI command handling both streaming and captured output modes
func runExec(args []string) {
	// Require "--" separator between query and command.
	sepIdx := -1

	for i, arg := range args {
		if arg == "--" {
			sepIdx = i
			break
		}
	}

	if sepIdx < 0 || sepIdx == len(args)-1 {
		if jsonOutput {
			writeJSONError("usage: repos exec <query> -- <command> [args...]\n       repos exec --all -- <command> [args...]", ExitError)
		}

		fmt.Fprintln(os.Stderr, "usage: repos exec <query> -- <command> [args...]")
		fmt.Fprintln(os.Stderr, "       repos exec --all -- <command> [args...]")
		os.Exit(ExitError)
	}

	queryArgs := args[:sepIdx]
	cmdArgs := args[sepIdx+1:]

	query := ""
	allRepos := false

	for _, arg := range queryArgs {
		switch arg {
		case "--all":
			allRepos = true
		default:
			query = arg
		}
	}

	if query == "" && !allRepos {
		if jsonOutput {
			writeJSONError("error: provide a query or --all", ExitError)
		}

		fmt.Fprintln(os.Stderr, "error: provide a query or --all")
		os.Exit(ExitError)
	}

	root := ReposRoot()

	var (
		repos []string
		err   error
	)

	if allRepos {
		repos, err = ListRepos(root, "")
	} else {
		repos, err = ListRepos(root, query)
	}

	if err != nil {
		if jsonOutput {
			writeJSONError(err.Error(), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(ExitError)
	}

	if len(repos) == 0 {
		if jsonOutput {
			writeJSONError("no repos found", ExitNotFound)
		}

		fmt.Fprintln(os.Stderr, "no repos found")
		os.Exit(ExitNotFound)
	}

	var results []ExecResult

	failures := 0

	for _, repo := range repos {
		repoPath := filepath.Join(root, repo)
		wtDir := defaultWorktreePath(repoPath)

		// Skip repos with no default worktree (e.g. tidy --create-default not yet run).
		if _, statErr := os.Stat(wtDir); statErr != nil {
			failures++

			if jsonOutput {
				results = append(results, ExecResult{
					Repo:     repo,
					Path:     wtDir,
					ExitCode: 1,
					Stderr:   "no default worktree",
				})
			} else {
				fmt.Fprintf(os.Stderr, "%s: no default worktree\n", repo)
			}

			continue
		}

		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = wtDir

		stdout, stderr := &strings.Builder{}, &strings.Builder{}
		if jsonOutput {
			// Capture stdout and stderr separately for structured output.
			cmd.Stdout = stdout
			cmd.Stderr = stderr
		} else {
			// Stream combined output with repo prefix.
			cmd.Stdout = &prefixWriter{prefix: repo + ": ", w: os.Stdout}
			cmd.Stderr = &prefixWriter{prefix: repo + ": ", w: os.Stderr}
		}

		exitCode := 0

		if runErr := cmd.Run(); runErr != nil {
			exitErr := &exec.ExitError{}
			if errors.As(runErr, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}

			failures++
		}

		if jsonOutput {
			results = append(results, ExecResult{
				Repo:     repo,
				Path:     wtDir,
				ExitCode: exitCode,
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
			})
		}
	}

	if jsonOutput {
		writeJSON(results)

		if failures > 0 {
			os.Exit(ExitError)
		}

		return
	}

	if failures > 0 {
		os.Exit(ExitError)
	}
}

// prefixWriter writes each line with a fixed prefix to the underlying writer.
type prefixWriter struct {
	prefix string
	w      *os.File
	buf    strings.Builder
}

func (pw *prefixWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.buf.Write(p)

	for {
		s := pw.buf.String()

		before, after, ok := strings.Cut(s, "\n")
		if !ok {
			break
		}

		line := before
		_, _ = fmt.Fprintf(pw.w, "%s%s\n", pw.prefix, line)
		pw.buf.Reset()
		pw.buf.WriteString(after)
	}

	return n, nil
}
