package main

import (
	"fmt"
	"os"
)

// jsonOutput is set by parseGlobalFlags when --json is present.
var jsonOutput bool

//nolint:gocyclo // CLI command dispatcher with one case per sub-command
func main() {
	var args []string

	jsonOutput, args = parseGlobalFlags(os.Args)

	if len(args) < 2 {
		printUsage()
		os.Exit(ExitError)
	}

	cmd := args[1]
	cmdArgs := args[2:]

	switch cmd {
	case "clone":
		runClone(cmdArgs)
	case "migrate":
		runMigrate(cmdArgs)
	case "list":
		runList(cmdArgs)
	case "audit":
		runAudit(cmdArgs)
	case "rm":
		runRemove(cmdArgs)
	case "path":
		runPath(cmdArgs)
	case "info":
		runInfo(cmdArgs)
	case "sync":
		runSync(cmdArgs)
	case "tidy":
		runTidy(cmdArgs)
	case "wt":
		runWorktree(cmdArgs)
	case "open":
		runOpen(cmdArgs)
	case "browse":
		runBrowse(cmdArgs)
	case "exec":
		runExec(cmdArgs)
	case "export":
		runExport(cmdArgs)
	case "import":
		runImport(cmdArgs)
	case "hook":
		runHookCmd(cmdArgs)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(ExitError)
	}
}

// parseGlobalFlags extracts --json from anywhere in args and returns
// the cleaned args slice without it.
func parseGlobalFlags(args []string) (jsonFlag bool, cleaned []string) {
	for _, arg := range args {
		if arg == "--json" {
			jsonFlag = true
		} else {
			cleaned = append(cleaned, arg)
		}
	}

	return jsonFlag, cleaned
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "repos — manage git repositories in ~/Dev/host/owner/repo layout\n\nUsage:\n  repos clone <url> [-b branch] [-q]  Clone into canonical path with bare+worktree layout\n  repos migrate [<path>]              Move+convert existing repo to canonical path\n  repos list [<query>] [-p]           List managed repos (filterable)\n  repos info <query>                  Show repo details (remote, branch, worktrees, status)\n  repos audit [<query>]               Check for unpushed/uncommitted work\n  repos sync [<query>] [-q]           Fetch + fast-forward default branch on all repos\n  tidy [<query>] [--prune] [--create-default]  Fix HEAD, detect stale worktrees\n  repos wt [<branch>] [-r query]      Manage worktrees (create/list/remove) for current repo\n  repos open [<query>]                Open repo in tmux session with nvim\n  repos browse [<query>]              Open repo in browser\n  repos exec <query> -- <cmd>         Run command across matching repos\n  export [-o file]              Export all repos to JSON manifest\n  repos import <file> [--dry-run]     Clone repos from JSON manifest\n  repos hook <list|add|rm> [args]     Manage lifecycle hooks\n  repos rm <query> [--force]          Remove a repo\n  repos path <query>                  Print repo's absolute path\n\nGlobal Flags:\n  --json    Output in JSON format (for agent/script consumption)\n\nExit Codes:\n  0  Success\n  1  Error (IO, git, invalid args)\n  2  Ambiguous query (multiple matches)\n  3  Not found (zero matches)\n\nEnvironment:\n  REPOS_ROOT    Base directory (default:")
}

func runList(cmdArgs []string) {
	query := ""
	fullPath := false

	for i := range cmdArgs {
		switch cmdArgs[i] {
		case "-p", "--full-path":
			fullPath = true
		default:
			query = cmdArgs[i]
		}
	}

	root := ReposRoot()

	repos, err := ListRepos(root, query)
	if err != nil {
		if jsonOutput {
			writeJSONError(err.Error(), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(ExitError)
	}

	if jsonOutput {
		entries := make([]RepoEntry, 0, len(repos))
		for _, r := range repos {
			entries = append(entries, NewRepoEntry(root, r))
		}

		writeJSON(entries)

		return
	}

	for _, r := range repos {
		if fullPath {
			fmt.Printf("%s/%s\n", root, r)
		} else {
			fmt.Println(r)
		}
	}
}
