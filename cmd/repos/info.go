package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RepoInfo holds detailed information about a single repository.
type RepoInfo struct {
	Path          string   `json:"path"`
	FullPath      string   `json:"fullPath"`
	Remote        string   `json:"remote"`
	DefaultBranch string   `json:"defaultBranch"`
	Worktrees     []string `json:"worktrees"`
	Dirty         bool     `json:"dirty"`
}

func runInfo(args []string) {
	if len(args) == 0 {
		if jsonOutput {
			writeJSONError("usage: repos info <query>", ExitError)
		}

		fmt.Fprintln(os.Stderr, "usage: repos info <query>")
		os.Exit(ExitError)
	}

	query := args[0]
	root := ReposRoot()

	repos, err := ListRepos(root, query)
	if err != nil {
		if jsonOutput {
			writeJSONError(err.Error(), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(ExitError)
	}

	switch len(repos) {
	case 0:
		if jsonOutput {
			writeJSONError(fmt.Sprintf("no repo matching %q", query), ExitNotFound)
		}

		fmt.Fprintf(os.Stderr, "no repo matching %q\n", query)
		os.Exit(ExitNotFound)
	case 1:
		// single match, proceed below
	default:
		if jsonOutput {
			writeJSONError(fmt.Sprintf("ambiguous query %q matches %d repos", query, len(repos)), ExitAmbiguous)
		}

		fmt.Fprintf(os.Stderr, "ambiguous query %q matches %d repos:\n", query, len(repos))

		for _, r := range repos {
			fmt.Fprintf(os.Stderr, "  %s\n", r)
		}

		os.Exit(ExitAmbiguous)
	}

	repoPath := filepath.Join(root, repos[0])
	info := getRepoInfo(root, repos[0], repoPath)

	if jsonOutput {
		writeJSON(info)
		return
	}

	// Human-readable output
	fmt.Println(info.Path)
	fmt.Printf("  remote:    %s\n", info.Remote)
	fmt.Printf("  branch:    %s\n", info.DefaultBranch)
	fmt.Printf("  worktrees: %s\n", strings.Join(info.Worktrees, ", "))

	if info.Dirty {
		fmt.Printf("  status:    dirty\n")
	} else {
		fmt.Printf("  status:    clean\n")
	}
}

// getRepoInfo gathers information about a single repository.
func getRepoInfo(_, relPath, repoPath string) RepoInfo {
	bareDir := filepath.Join(repoPath, ".git")

	info := RepoInfo{
		Path:     relPath,
		FullPath: repoPath,
	}

	// Remote URL
	if url, err := gitOutput(bareDir, "remote", "get-url", "origin"); err == nil {
		info.Remote = strings.TrimSpace(url)
	}

	// Default branch
	info.DefaultBranch = detectDefaultBranch(bareDir)

	// Worktrees
	out, err := gitOutput(bareDir, "worktree", "list", "--porcelain")
	if err == nil {
		for line := range strings.SplitSeq(out, "\n") {
			if !strings.HasPrefix(line, "worktree ") {
				continue
			}

			wtPath := strings.TrimPrefix(line, "worktree ")
			if wtPath == bareDir {
				continue
			}

			info.Worktrees = append(info.Worktrees, filepath.Base(wtPath))
		}
	}

	// Dirty check — any worktree has uncommitted changes
	for _, wt := range info.Worktrees {
		wtPath := filepath.Join(repoPath, wt)

		statusOut, err := gitOutput(wtPath, "status", "--porcelain")
		if err != nil {
			continue
		}

		if strings.TrimSpace(statusOut) != "" {
			info.Dirty = true
			break
		}
	}

	return info
}
