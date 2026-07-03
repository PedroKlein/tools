package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func runPath(args []string) {
	if len(args) == 0 {
		if jsonOutput {
			writeJSONError("usage: repos path <query>", ExitError)
		}

		fmt.Fprintln(os.Stderr, "usage: repos path <query>")
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
		result := filepath.Join(root, repos[0])
		if jsonOutput {
			writeJSON(struct {
				Path string `json:"path"`
			}{Path: result})

			return
		}

		fmt.Println(result)
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
}
