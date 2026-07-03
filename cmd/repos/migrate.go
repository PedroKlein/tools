package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//nolint:gocyclo // complex migration command with validation, confirmation, and multiple git steps
func runMigrate(args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving path: %v\n", err)
		os.Exit(1)
	}

	// Check it exists
	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "error: %s is not a directory\n", absPath)
		os.Exit(1)
	}

	// Determine if it's a standard clone (.git/) or already bare-worktree layout
	gitDir := filepath.Join(absPath, ".git")

	gitInfo, err := os.Stat(gitDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s is not a git repository (no .git found)\n", absPath)
		os.Exit(1)
	}

	// Check if already in our bare+worktree layout
	if isBareRepo(absPath) {
		fmt.Fprintln(os.Stderr, "already in bare/worktree layout — use repos clone for fresh repos")
		os.Exit(1)
	}

	// Check if it's a bare repo (.git is a directory with normal bare content)
	// or a standard clone (.git is a directory with worktree)
	if !gitInfo.IsDir() {
		// .git is a file — this is inside a worktree, not a root repo
		fmt.Fprintln(os.Stderr, "error: this appears to be inside a worktree, not a repo root")
		os.Exit(1)
	}

	// Check for existing worktrees (can't safely migrate those)
	wtOut, err := gitOutput(absPath, "worktree", "list", "--porcelain")
	if err == nil {
		wtCount := 0

		for line := range strings.SplitSeq(wtOut, "\n") {
			if strings.HasPrefix(line, "worktree ") {
				wtCount++
			}
		}

		if wtCount > 1 {
			fmt.Fprintln(os.Stderr, "error: repo has existing worktrees — cannot auto-migrate")
			fmt.Fprintln(os.Stderr, "  remove worktrees first, or use 'repos clone' for a fresh start")
			os.Exit(1)
		}
	}

	// Read origin remote URL
	originURL, err := gitOutput(absPath, "config", "--get", "remote.origin.url")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot read origin remote: %v\n", err)
		os.Exit(1)
	}

	originURL = strings.TrimSpace(originURL)

	// Parse URL to determine canonical path
	host, owner, repo, err := ParseRemoteURL(originURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing origin URL %q: %v\n", originURL, err)
		os.Exit(1)
	}

	root := ReposRoot()
	targetDir := filepath.Join(root, host, owner, repo)

	// Check target doesn't already exist
	if _, statErr := os.Stat(targetDir); statErr == nil {
		fmt.Fprintf(os.Stderr, "error: target already exists: %s\n", targetDir)
		os.Exit(1)
	}

	// Detect current branch
	currentBranch, err := gitOutput(absPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		currentBranch = fallbackDefaultBranch
	}

	currentBranch = strings.TrimSpace(currentBranch)

	// Show plan and confirm
	fmt.Printf("Migrate plan:\n")
	fmt.Printf("  from: %s\n", absPath)
	fmt.Printf("  to:   %s\n", targetDir)
	fmt.Printf("  remote: %s\n", originURL)
	fmt.Printf("  layout: .git/ → bare .git/ + %s/ (worktree)\n", currentBranch)
	fmt.Print("\nProceed? [y/N] ")

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')

	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("canceled")
		return
	}

	// Execute migration
	if err := doMigrate(absPath, targetDir, currentBranch); err != nil {
		fmt.Fprintf(os.Stderr, "error during migration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ Migrated to %s/%s/%s\n", host, owner, repo)
	fmt.Printf("  └── %s/  (worktree)\n", currentBranch)
}

// doMigrate performs the actual migration:
// 1. Create target dir
// 2. Move .git → target/.git
// 3. Configure as bare
// 4. Create worktree for current branch
func doMigrate(source, target, branch string) error {
	// Create target directory
	if err := os.MkdirAll(target, 0o750); err != nil {
		return fmt.Errorf("creating target dir: %w", err)
	}

	bareDir := filepath.Join(target, ".git")

	// Move .git directory to target/.git
	sourceGit := filepath.Join(source, ".git")
	if err := os.Rename(sourceGit, bareDir); err != nil {
		return fmt.Errorf("moving .git to bare .git: %w", err)
	}

	// Mark as bare repo
	if err := gitCmd(bareDir, "config", "core.bare", "true"); err != nil {
		return fmt.Errorf("setting core.bare: %w", err)
	}

	// Ensure fetch refspec is set
	_ = gitCmd(bareDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")

	// Create worktree
	worktreeDir := filepath.Join(target, branch)
	if err := gitCmd(bareDir, "worktree", "add", worktreeDir, branch); err != nil {
		return fmt.Errorf("creating worktree: %w", err)
	}

	// Set tracking
	_ = gitCmd(bareDir, "branch", "--set-upstream-to=origin/"+branch, branch)

	// Remove old source directory (now empty except for non-git files)
	// List remaining files in source
	entries, err := os.ReadDir(source)
	if err == nil && len(entries) == 0 {
		_ = os.Remove(source)
		// Clean up empty parents
		cleanEmptyParents(filepath.Dir(source), ReposRoot())
	} else if err == nil {
		fmt.Printf("\n  note: old directory still has files: %s\n", source)

		for _, e := range entries {
			fmt.Printf("    %s\n", e.Name())
		}
	}

	return nil
}
