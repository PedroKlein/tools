package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runRemove(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: repos rm <query> [--force]")
		os.Exit(1)
	}

	var query string

	force := false

	for _, arg := range args {
		switch arg {
		case "--force", "-f":
			force = true
		default:
			query = arg
		}
	}

	if query == "" {
		fmt.Fprintln(os.Stderr, "usage: repos rm <query> [--force]")
		os.Exit(1)
	}

	root := ReposRoot()

	repos, err := ListRepos(root, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	switch len(repos) {
	case 0:
		fmt.Fprintf(os.Stderr, "no repo matching %q\n", query)
		os.Exit(1)
	case 1:
		// proceed
	default:
		fmt.Fprintf(os.Stderr, "ambiguous query %q matches %d repos:\n", query, len(repos))

		for _, r := range repos {
			fmt.Fprintf(os.Stderr, "  %s\n", r)
		}

		os.Exit(1)
	}

	repoPath := filepath.Join(root, repos[0])

	if !force {
		fmt.Printf("Remove %s? [y/N] ", repos[0])

		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')

		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("canceled")
			return
		}
	}

	if err := os.RemoveAll(repoPath); err != nil {
		fmt.Fprintf(os.Stderr, "error removing: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("removed %s\n", repos[0])

	// Clean up empty parent directories
	ownerDir := filepath.Dir(repoPath)
	cleanEmptyParents(ownerDir, root)
}

// cleanEmptyParents removes empty directories up to (but not including) root.
func cleanEmptyParents(dir, root string) {
	for dir != root && dir != "/" {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}

		_ = os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}
