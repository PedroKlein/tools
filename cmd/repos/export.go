package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManifestEntry represents a single repo in the export manifest.
type ManifestEntry struct {
	URL           string `json:"url"`
	Host          string `json:"host"`
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	DefaultBranch string `json:"defaultBranch"`
}

func runExport(args []string) {
	outFile := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o", "--output":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: -o requires a file path")
				os.Exit(ExitError)
			}

			i++
			outFile = args[i]
		}
	}

	root := ReposRoot()

	repos, err := ListRepos(root, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(ExitError)
	}

	var entries []ManifestEntry

	for _, repo := range repos {
		repoPath := filepath.Join(root, repo)
		bareDir := filepath.Join(repoPath, ".git")

		url, urlErr := gitOutput(bareDir, "remote", "get-url", "origin")
		if urlErr != nil {
			continue // skip repos without a remote
		}

		url = strings.TrimSpace(url)

		parts := strings.SplitN(repo, string(filepath.Separator), 3)

		entry := ManifestEntry{
			URL:           url,
			DefaultBranch: detectDefaultBranch(bareDir),
		}
		if len(parts) >= 1 {
			entry.Host = parts[0]
		}

		if len(parts) >= 2 {
			entry.Owner = parts[1]
		}

		if len(parts) >= 3 {
			entry.Repo = parts[2]
		}

		entries = append(entries, entry)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling: %v\n", err)
		os.Exit(ExitError)
	}

	data = append(data, '\n')

	if outFile != "" {
		if err := os.WriteFile(outFile, data, 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outFile, err)
			os.Exit(ExitError)
		}

		fmt.Printf("exported %d repos to %s\n", len(entries), outFile)
	} else {
		_, _ = os.Stdout.Write(data)
	}
}

//nolint:gocyclo // complex import handler with validation, clone, and worktree setup
func runImport(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: repos import <manifest.json> [--dry-run]")
		os.Exit(ExitError)
	}

	var manifestFile string

	dryRun := false

	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		default:
			if manifestFile == "" {
				manifestFile = arg
			}
		}
	}

	data, err := os.ReadFile(manifestFile) //nolint:gosec // user-provided file path is expected for this command
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", manifestFile, err)
		os.Exit(ExitError)
	}

	var entries []ManifestEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing manifest: %v\n", err)
		os.Exit(ExitError)
	}

	root := ReposRoot()

	var cloned, skipped, failed int

	for _, entry := range entries {
		repoDir := filepath.Join(root, entry.Host, entry.Owner, entry.Repo)
		gitDir := filepath.Join(repoDir, ".git")

		// Skip already-managed repos.
		if _, err := os.Stat(gitDir); err == nil {
			fmt.Printf("⊜ %s/%s/%s (exists)\n", entry.Host, entry.Owner, entry.Repo)

			skipped++

			continue
		}

		if dryRun {
			fmt.Printf("  would clone %s → %s/%s/%s\n", entry.URL, entry.Host, entry.Owner, entry.Repo)

			cloned++

			continue
		}

		fmt.Printf("↓ %s/%s/%s ...\n", entry.Host, entry.Owner, entry.Repo)

		if err := os.MkdirAll(repoDir, 0o750); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)

			failed++

			continue
		}

		if err := gitCmdQuiet(repoDir, "clone", "--bare", entry.URL, ".git"); err != nil {
			if removeErr := os.RemoveAll(repoDir); removeErr != nil {
				fmt.Fprintf(os.Stderr, "  warning: cleanup failed: %v\n", removeErr)
			}

			fmt.Fprintf(os.Stderr, "  error cloning: %v\n", err)

			failed++

			continue
		}

		// Configure fetch refspec and refresh remote refs.
		_ = gitCmdQuiet(gitDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
		_ = gitCmdQuiet(gitDir, "fetch", "origin")

		// Create default branch worktree.
		defaultBranch := detectDefaultBranch(gitDir)

		worktreeDir := filepath.Join(repoDir, defaultBranch)

		if err := addDefaultWorktree(gitDir, worktreeDir, defaultBranch); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: worktree add for %q failed: %v\n", defaultBranch, err)
			fmt.Fprintf(os.Stderr, "           bare repo left at %s; run: repos wt add %s\n", gitDir, defaultBranch)

			cloned++

			continue
		}

		_ = gitCmdQuiet(gitDir, "branch", "--set-upstream-to=origin/"+defaultBranch, defaultBranch)

		fmt.Printf("  ✓ %s/\n", defaultBranch)

		cloned++
	}

	fmt.Println()

	if dryRun {
		fmt.Printf("%d would be cloned, %d already exist\n", cloned, skipped)
	} else {
		fmt.Printf("%d cloned, %d skipped", cloned, skipped)

		if failed > 0 {
			fmt.Printf(", %d failed", failed)
		}

		fmt.Println()
	}

	if failed > 0 {
		os.Exit(ExitError)
	}
}
