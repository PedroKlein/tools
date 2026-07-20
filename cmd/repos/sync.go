package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SyncEntry is the JSON representation of a repo's sync result.
type SyncEntry struct {
	Path    string `json:"path"`
	Status  string `json:"status"`            // "updated", "up-to-date", "diverged", "error"
	Branch  string `json:"branch,omitempty"`  // for updated, diverged
	Commits int    `json:"commits,omitempty"` // for updated
	Detail  string `json:"detail,omitempty"`  // for error
}

//nolint:gocognit,gocyclo // complex CLI command with multiple sync states and hook execution
func runSync(args []string) {
	query := ""
	quiet := false

	for _, arg := range args {
		switch arg {
		case "-q", "--quiet":
			quiet = true
		default:
			query = arg
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

	if len(repos) == 0 {
		if jsonOutput {
			writeJSONError("no repos found", ExitNotFound)
		}

		fmt.Fprintln(os.Stderr, "no repos found")
		os.Exit(ExitNotFound)
	}

	var (
		entries                              = make([]SyncEntry, 0, len(repos))
		updated, upToDate, diverged, errored int
	)

	for _, repo := range repos {
		repoPath := filepath.Join(root, repo)
		bareDir := filepath.Join(repoPath, ".git")

		var result syncResult
		if isBareRepo(repoPath) {
			result = syncRepo(bareDir)
		} else {
			result = syncFlatRepo(repoPath)
		}

		entry := SyncEntry{Path: repo}

		switch result.status {
		case syncUpdated:
			updated++
			entry.Status = "updated"
			entry.Branch = extractBranch(result.detail)

			entry.Commits = extractCommits(result.detail)
			if !quiet && !jsonOutput {
				fmt.Printf("↑ %s (%s)\n", repo, result.detail)
			}
		case syncUpToDate:
			upToDate++
			entry.Status = "up-to-date"
		case syncDiverged:
			diverged++
			entry.Status = "diverged"

			entry.Branch = result.detail
			if !quiet && !jsonOutput {
				fmt.Printf("⇋ %s (diverged: %s)\n", repo, result.detail)
			}
		case syncError:
			errored++
			entry.Status = "error"

			entry.Detail = result.detail
			if !quiet && !jsonOutput {
				fmt.Printf("✗ %s (%s)\n", repo, result.detail)
			}
		}

		entries = append(entries, entry)
	}

	if jsonOutput {
		writeJSON(entries)

		if errored > 0 || diverged > 0 {
			os.Exit(ExitError)
		}

		return
	}

	// Human summary
	if !quiet {
		fmt.Println()
	}

	fmt.Printf("%d updated, %d up-to-date", updated, upToDate)

	if diverged > 0 {
		fmt.Printf(", %d diverged", diverged)
	}

	if errored > 0 {
		fmt.Printf(", %d errors", errored)
	}

	fmt.Println()

	// Run post-sync hooks on updated repos
	if updated > 0 {
		runPostSyncHooks(root, entries)
	}

	if errored > 0 || diverged > 0 {
		os.Exit(ExitError)
	}
}

// runPostSyncHooks collects updated-repo metadata and fires the post-sync hook event.
func runPostSyncHooks(root string, entries []SyncEntry) {
	var hookRepos []HookRepoInfo

	for _, e := range entries {
		if e.Status != "updated" {
			continue
		}

		parts := strings.SplitN(e.Path, string(filepath.Separator), 3)

		info := HookRepoInfo{
			Path:   filepath.Join(root, e.Path),
			Branch: e.Branch,
		}
		if len(parts) >= 1 {
			info.Host = parts[0]
		}

		if len(parts) >= 2 {
			info.Owner = parts[1]
		}

		if len(parts) >= 3 {
			info.Name = parts[2]
		}

		hookRepos = append(hookRepos, info)
	}

	runHooksForEvent("post-sync", hookRepos)
}

// extractBranch extracts the branch name from a sync detail string like "main +5 commits".
func extractBranch(detail string) string {
	parts := strings.Fields(detail)
	if len(parts) > 0 {
		return parts[0]
	}

	return ""
}

// extractCommits extracts the commit count from a sync detail string like "main +5 commits".
func extractCommits(detail string) int {
	parts := strings.Fields(detail)
	if len(parts) >= 2 {
		s := strings.TrimPrefix(parts[1], "+")
		n := 0

		for _, c := range s {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			}
		}

		return n
	}

	return 0
}

type syncStatus int

const (
	syncUpdated syncStatus = iota
	syncUpToDate
	syncDiverged
	syncError
)

type syncResult struct {
	status syncStatus
	detail string
}

func syncRepo(bareDir string) syncResult {
	// Fetch from origin
	if err := gitCmdQuiet(bareDir, "fetch", "origin", "--prune"); err != nil {
		return syncResult{syncError, fmt.Sprintf("fetch failed: %v", err)}
	}

	// Detect default branch
	branch := detectDefaultBranch(bareDir)

	// Get local and remote refs
	localRef, err := gitOutput(bareDir, "rev-parse", "refs/heads/"+branch)
	if err != nil {
		return syncResult{syncError, "no local branch " + branch}
	}

	localRef = strings.TrimSpace(localRef)

	remoteRef, err := gitOutput(bareDir, "rev-parse", "refs/remotes/origin/"+branch)
	if err != nil {
		return syncResult{syncError, "no remote branch origin/" + branch}
	}

	remoteRef = strings.TrimSpace(remoteRef)

	// Already up to date
	if localRef == remoteRef {
		return syncResult{syncUpToDate, ""}
	}

	// Check if fast-forward is possible (local is ancestor of remote)
	mergeBase, err := gitOutput(bareDir, "merge-base", localRef, remoteRef)
	if err != nil {
		return syncResult{syncError, "cannot compute merge-base"}
	}

	mergeBase = strings.TrimSpace(mergeBase)

	if mergeBase != localRef {
		return syncResult{syncDiverged, branch}
	}

	// Fast-forward: update the local branch ref
	if err := gitCmdQuiet(bareDir, "update-ref", "refs/heads/"+branch, remoteRef); err != nil {
		return syncResult{syncError, fmt.Sprintf("update-ref failed: %v", err)}
	}

	// Count commits advanced
	countOut, _ := gitOutput(bareDir, "rev-list", "--count", localRef+".."+remoteRef)
	count := strings.TrimSpace(countOut)

	return syncResult{syncUpdated, fmt.Sprintf("%s +%s commits", branch, count)}
}

// gitCmdQuiet runs a git command without printing stdout/stderr to the parent
// process. It captures combined output and folds it into the returned error
// message when git exits non-zero, so callers that surface the error get a
// useful diagnostic instead of a bare "exit status 128". Callers that discard
// the error still see no output.
func gitCmdQuiet(dir string, args ...string) error {
	cmd := gitCommand(dir, args...)

	var buf bytes.Buffer

	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		out := strings.TrimSpace(buf.String())
		if out == "" {
			return fmt.Errorf("git %v: %w", args, err)
		}

		return fmt.Errorf("git %v: %w: %s", args, err, out)
	}

	return nil
}

// syncFlatRepo syncs a non-bare (flat) repo using fetch + ff-only merge.
func syncFlatRepo(repoPath string) syncResult {
	// Fetch from origin
	if err := gitCmdQuiet(repoPath, "fetch", "origin", "--prune"); err != nil {
		return syncResult{syncError, fmt.Sprintf("fetch failed: %v", err)}
	}

	// Detect current branch
	branchOut, err := gitOutput(repoPath, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return syncResult{syncError, "cannot determine current branch"}
	}

	branch := strings.TrimSpace(branchOut)

	// Get local and remote refs
	localRef, err := gitOutput(repoPath, "rev-parse", "HEAD")
	if err != nil {
		return syncResult{syncError, "cannot get HEAD"}
	}

	localRef = strings.TrimSpace(localRef)

	remoteRef, err := gitOutput(repoPath, "rev-parse", "refs/remotes/origin/"+branch)
	if err != nil {
		return syncResult{syncError, "no remote branch origin/" + branch}
	}

	remoteRef = strings.TrimSpace(remoteRef)

	if localRef == remoteRef {
		return syncResult{syncUpToDate, ""}
	}

	// Check if fast-forward is possible
	mergeBase, err := gitOutput(repoPath, "merge-base", localRef, remoteRef)
	if err != nil {
		return syncResult{syncError, "cannot compute merge-base"}
	}

	mergeBase = strings.TrimSpace(mergeBase)

	if mergeBase != localRef {
		return syncResult{syncDiverged, branch}
	}

	// Fast-forward merge
	if err := gitCmdQuiet(repoPath, "merge", "--ff-only", "origin/"+branch); err != nil {
		return syncResult{syncError, fmt.Sprintf("ff-only merge failed: %v", err)}
	}

	// Count commits advanced
	countOut, _ := gitOutput(repoPath, "rev-list", "--count", localRef+".."+remoteRef)
	count := strings.TrimSpace(countOut)

	return syncResult{syncUpdated, fmt.Sprintf("%s +%s commits", branch, count)}
}
