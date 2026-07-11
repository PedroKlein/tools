package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReposRoot returns the base directory for managed repos.
// Priority: REPOS_ROOT env var > default ~/Dev.
func ReposRoot() string {
	if root := os.Getenv("REPOS_ROOT"); root != "" {
		return expandTilde(root)
	}

	return filepath.Join(homeDir(), "Dev")
}

// homeDir returns the user's home directory.
func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback
		return os.Getenv("HOME")
	}

	return home
}

// expandTilde expands a leading ~ to the home directory.
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir(), path[2:])
	}

	if path == "~" {
		return homeDir()
	}

	return path
}

// isGitRepo checks if dir contains a git repository by looking for
// .git/ as a directory containing a config file. Returns false on any error
// or if .git is a file (worktree pointer).
func isGitRepo(dir string) bool {
	gitPath := filepath.Join(dir, ".git")

	info, err := os.Stat(gitPath)
	if err != nil || !info.IsDir() {
		return false
	}

	_, err = os.Stat(filepath.Join(gitPath, "config"))

	return err == nil
}

// isBareRepo checks if dir contains a bare git repository by looking for
// .git/config with core.bare = true. Returns false on any error or if
// the directory is not a bare repo. The .git entry must be a directory
// (not a file, which would be a worktree pointer).
func isBareRepo(dir string) bool {
	gitPath := filepath.Join(dir, ".git")

	info, err := os.Stat(gitPath)
	if err != nil || !info.IsDir() {
		return false
	}

	f, err := os.Open(filepath.Join(gitPath, "config")) //nolint:gosec // path is constructed from a validated repo directory
	if err != nil {
		return false
	}
	defer f.Close() //nolint:errcheck // read-only file, Close error is irrelevant

	inCore := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Section header
		if strings.HasPrefix(line, "[") {
			inCore = strings.EqualFold(line, "[core]")
			continue
		}

		if !inCore {
			continue
		}

		// Normalise: remove spaces around '='
		normalized := strings.ReplaceAll(line, " ", "")
		if normalized == "bare=true" {
			return true
		}
	}

	return false
}

// detectCurrentRepo determines which managed repo the CWD is inside.
// It uses git to find the common git dir, then validates it's under REPOS_ROOT.
// Returns the relative path (host/owner/repo) or error if not in a managed repo.
func detectCurrentRepo() (string, error) {
	out, err := gitOutput(".", "rev-parse", "--git-common-dir")
	if err != nil {
		return "", errors.New("not inside a git repository")
	}

	gitCommonDir := strings.TrimSpace(out)

	// Make absolute
	if !filepath.IsAbs(gitCommonDir) {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return "", fmt.Errorf("getting working directory: %w", cwdErr)
		}

		gitCommonDir = filepath.Join(cwd, gitCommonDir)
	}

	gitCommonDir = filepath.Clean(gitCommonDir)

	// The repo root is the parent of .git/
	repoRoot := filepath.Dir(gitCommonDir)

	// Validate it's under REPOS_ROOT
	root := ReposRoot()

	rel, err := filepath.Rel(root, repoRoot)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("repo %s is not under REPOS_ROOT (%s)", repoRoot, root)
	}

	// Validate it's at the right depth (host/owner/repo = 3 parts)
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) != 3 {
		return "", fmt.Errorf("repo path %s is not at expected depth (host/owner/repo)", rel)
	}

	return rel, nil
}

// defaultWorktreePath returns the absolute path to the default branch worktree.
func defaultWorktreePath(repoPath string) string {
	bareDir := filepath.Join(repoPath, ".git")
	branch := detectDefaultBranch(bareDir)

	return filepath.Join(repoPath, branch)
}

// resolveRepo resolves a repo path either from an explicit query or from CWD.
// Returns (absolute repo path, relative path, error).
func resolveRepo(query string) (repoPath, relPath string, err error) {
	if query != "" {
		root := ReposRoot()

		repos, listErr := ListRepos(root, query)
		if listErr != nil {
			return "", "", listErr
		}

		switch len(repos) {
		case 0:
			return "", "", fmt.Errorf("no repo matching %q", query)
		case 1:
			return filepath.Join(root, repos[0]), repos[0], nil
		default:
			return "", "", fmt.Errorf("ambiguous query %q matches %d repos", query, len(repos))
		}
	}

	// No query — detect from CWD
	rel, err := detectCurrentRepo()
	if err != nil {
		return "", "", err
	}

	root := ReposRoot()

	return filepath.Join(root, rel), rel, nil
}
