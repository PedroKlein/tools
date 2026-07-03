package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WorktreeEntry is the JSON representation of a single worktree.
type WorktreeEntry struct {
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

func runWorktree(args []string) {
	// Parse flags: -r <query>, --rm, --force
	var (
		repoQuery string
		rmBranch  string
		branch    string
	)

	force := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-r", "--repo":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: -r requires a repo query")
				os.Exit(ExitError)
			}

			i++
			repoQuery = args[i]
		case "--rm", "--remove":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --rm requires a branch name")
				os.Exit(ExitError)
			}

			i++
			rmBranch = args[i]
		case "--force", "-f":
			force = true
		default:
			if branch == "" {
				branch = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "error: unexpected argument %q\n", args[i])
				os.Exit(ExitError)
			}
		}
	}

	repoPath, relPath, err := resolveRepo(repoQuery)
	if err != nil {
		if jsonOutput {
			writeJSONError(err.Error(), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(ExitError)
	}

	switch {
	case rmBranch != "":
		wtRemove(repoPath, relPath, rmBranch, force)
	case branch != "":
		wtCreate(repoPath, relPath, branch)
	default:
		wtList(repoPath)
	}
}

// wtList prints all worktrees for the repo (excluding the bare root itself).
func wtList(repoPath string) {
	bareDir := filepath.Join(repoPath, ".git")

	out, err := gitOutput(bareDir, "worktree", "list", "--porcelain")
	if err != nil {
		if jsonOutput {
			writeJSONError(fmt.Sprintf("cannot list worktrees: %v", err), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error: cannot list worktrees: %v\n", err)
		os.Exit(ExitError)
	}

	entries := parseWorktrees(out, bareDir)

	if jsonOutput {
		writeJSON(entries)
		return
	}

	for _, e := range entries {
		fmt.Printf("%-30s %s\n", e.Branch, e.Path)
	}
}

// wtCreate creates a new worktree for branch, tracking origin if the remote
// branch exists, otherwise creating a new local branch.
func wtCreate(repoPath, _, branch string) {
	bareDir := filepath.Join(repoPath, ".git")
	worktreeDir := filepath.Join(repoPath, branch)

	// Check if worktree already exists on disk
	if _, err := os.Stat(worktreeDir); err == nil {
		if jsonOutput {
			writeJSONError("worktree already exists: "+worktreeDir, ExitError)
		}

		fmt.Fprintf(os.Stderr, "error: worktree already exists: %s\n", worktreeDir)
		os.Exit(ExitError)
	}

	// Check if remote branch exists
	_, remoteErr := gitOutput(bareDir, "rev-parse", "--verify", "refs/remotes/origin/"+branch)
	remoteExists := remoteErr == nil

	// Check if local branch already exists
	_, localErr := gitOutput(bareDir, "rev-parse", "--verify", "refs/heads/"+branch)
	localExists := localErr == nil

	var addErr error
	if localExists {
		// Branch exists locally — just create worktree for it
		addErr = gitCmdQuiet(bareDir, "worktree", "add", worktreeDir, branch)
	} else if remoteExists {
		// Track the remote branch
		addErr = gitCmdQuiet(bareDir, "worktree", "add", "--track", "-b", branch, worktreeDir, "origin/"+branch)
		if addErr == nil {
			_ = gitCmdQuiet(bareDir, "branch", "--set-upstream-to=origin/"+branch, branch)
		}
	} else {
		// New local branch
		addErr = gitCmdQuiet(bareDir, "worktree", "add", "-b", branch, worktreeDir)
	}

	if addErr != nil {
		if jsonOutput {
			writeJSONError(fmt.Sprintf("cannot create worktree: %v", addErr), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error: cannot create worktree: %v\n", addErr)
		os.Exit(ExitError)
	}

	if jsonOutput {
		writeJSON(WorktreeEntry{Branch: branch, Path: worktreeDir})
		return
	}

	// Print path for cd composition: cd $(repos wt feat-x)
	fmt.Println(worktreeDir)
}

// wtRemove removes the worktree for branch and deletes the local branch.
func wtRemove(repoPath, _, branch string, force bool) {
	bareDir := filepath.Join(repoPath, ".git")
	worktreeDir := filepath.Join(repoPath, branch)

	// Verify worktree exists
	if _, err := os.Stat(worktreeDir); err != nil {
		if jsonOutput {
			writeJSONError(fmt.Sprintf("no worktree for branch %q", branch), ExitNotFound)
		}

		fmt.Fprintf(os.Stderr, "error: no worktree for branch %q at %s\n", branch, worktreeDir)
		os.Exit(ExitNotFound)
	}

	// Dirty check unless --force
	if !force {
		statusOut, err := gitOutput(worktreeDir, "status", "--porcelain")
		if err == nil && strings.TrimSpace(statusOut) != "" {
			if jsonOutput {
				writeJSONError(fmt.Sprintf("worktree %q is dirty; use --force to remove anyway", branch), ExitError)
			}

			fmt.Fprintf(os.Stderr, "error: worktree %q is dirty; use --force to remove anyway\n", branch)
			os.Exit(ExitError)
		}
	}

	removeArgs := []string{"worktree", "remove", worktreeDir}
	if force {
		removeArgs = append(removeArgs, "--force")
	}

	if err := gitCmdQuiet(bareDir, removeArgs...); err != nil {
		if jsonOutput {
			writeJSONError(fmt.Sprintf("cannot remove worktree: %v", err), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error: cannot remove worktree: %v\n", err)
		os.Exit(ExitError)
	}

	// Delete the local branch (best-effort)
	_ = gitCmdQuiet(bareDir, "branch", "-D", branch)

	if jsonOutput {
		writeJSON(struct {
			Removed string `json:"removed"`
			Path    string `json:"path"`
		}{Removed: branch, Path: worktreeDir})

		return
	}

	fmt.Printf("removed %s\n", branch)
}

// parseWorktrees parses `git worktree list --porcelain` output into entries.
// Skips the bare repo entry (identified by the bareDir path).
func parseWorktrees(out, bareDir string) []WorktreeEntry {
	var (
		entries     []WorktreeEntry
		currentPath string
	)

	for line := range strings.SplitSeq(out, "\n") {
		if after, ok := strings.CutPrefix(line, "worktree "); ok {
			currentPath = after
			continue
		}

		if strings.HasPrefix(line, "branch ") {
			// Skip the bare repo itself
			if currentPath == bareDir {
				continue
			}

			branchRef := strings.TrimPrefix(line, "branch ")
			branchName := strings.TrimPrefix(branchRef, "refs/heads/")
			entries = append(entries, WorktreeEntry{
				Branch: branchName,
				Path:   currentPath,
			})
		}
	}

	return entries
}
