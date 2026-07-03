package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TidyResult is the JSON representation of a repo's tidy findings.
type TidyResult struct {
	Path     string        `json:"path"`
	Findings []TidyFinding `json:"findings"`
	Actions  []TidyAction  `json:"actions,omitempty"`
}

// TidyFinding represents a single tidy issue found in a repo.
type TidyFinding struct {
	Type     string `json:"type"`               // "head_mismatch", "stale_worktree", "no_default_worktree"
	Current  string `json:"current,omitempty"`  // for head_mismatch
	Expected string `json:"expected,omitempty"` // for head_mismatch
	Worktree string `json:"worktree,omitempty"` // for stale_worktree
	Branch   string `json:"branch,omitempty"`   // for stale_worktree, no_default_worktree
	Dirty    bool   `json:"dirty,omitempty"`    // for stale_worktree
}

// TidyAction represents an action taken during tidy.
type TidyAction struct {
	Type   string `json:"type"` // "reset_head", "prune_worktree", "create_worktree", "skip_dirty"
	Detail string `json:"detail,omitempty"`
}

// TidyOptions controls what tidy does.
type TidyOptions struct {
	Prune         bool
	CreateDefault bool
	Quiet         bool
}

//nolint:gocognit,gocyclo // complex CLI command with multiple tidy operations and output modes
func runTidy(args []string) {
	query := ""
	opts := TidyOptions{}

	for _, arg := range args {
		switch arg {
		case "--prune":
			opts.Prune = true
		case "--create-default":
			opts.CreateDefault = true
		case "-q", "--quiet":
			opts.Quiet = true
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

	results := make([]TidyResult, 0, len(repos))

	issueCount := 0

	for _, repo := range repos {
		repoPath := filepath.Join(root, repo)
		result := tidyRepo(repoPath, opts)
		result.Path = repo

		if len(result.Findings) > 0 {
			issueCount++
		}

		results = append(results, result)
	}

	if jsonOutput {
		writeJSON(results)
		return
	}

	// Human-readable output
	for _, r := range results {
		if len(r.Findings) == 0 {
			continue
		}

		fmt.Printf("⚠ %s\n", r.Path)

		for _, f := range r.Findings {
			fmt.Printf("  %s\n", formatTidyFinding(f))
		}

		for _, a := range r.Actions {
			fmt.Printf("  ✓ %s\n", formatTidyAction(a))
		}
	}

	if issueCount == 0 {
		if !opts.Quiet {
			fmt.Printf("✓ All %d repos tidy\n", len(repos))
		}
	} else {
		fmt.Printf("\n%d/%d repos had issues\n", issueCount, len(repos))
	}

	// Run post-tidy hooks on repos that had issues fixed
	var hookRepos []HookRepoInfo

	for _, r := range results {
		if len(r.Actions) == 0 {
			continue
		}

		parts := strings.SplitN(r.Path, string(filepath.Separator), 3)

		info := HookRepoInfo{
			Path: filepath.Join(root, r.Path),
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

		info.Branch = detectDefaultBranch(filepath.Join(root, r.Path, ".git"))
		hookRepos = append(hookRepos, info)
	}

	if len(hookRepos) > 0 {
		runHooksForEvent("post-tidy", hookRepos)
	}
}

//nolint:gocyclo // tidyRepo analyses HEAD, stale worktrees, and default-branch worktree in one pass
func tidyRepo(repoPath string, opts TidyOptions) TidyResult {
	bareDir := filepath.Join(repoPath, ".git")
	result := TidyResult{}

	// 1. Check HEAD vs origin/HEAD mismatch
	defaultBranch := detectDefaultBranch(bareDir)
	currentHead := readLocalHead(bareDir)

	if currentHead != "" && currentHead != defaultBranch {
		result.Findings = append(result.Findings, TidyFinding{
			Type:     "head_mismatch",
			Current:  currentHead,
			Expected: defaultBranch,
		})
		// Always fix HEAD mismatch — it's safe and non-destructive
		if err := gitCmdQuiet(bareDir, "symbolic-ref", "HEAD", "refs/heads/"+defaultBranch); err == nil {
			result.Actions = append(result.Actions, TidyAction{
				Type:   "reset_head",
				Detail: "HEAD → refs/heads/" + defaultBranch,
			})
		}
	}

	// 2. Detect stale worktrees (branch deleted from remote)
	out, err := gitOutput(bareDir, "worktree", "list", "--porcelain")
	if err != nil {
		return result
	}

	var worktrees []struct {
		path   string
		branch string
	}

	var currentWT string

	for line := range strings.SplitSeq(out, "\n") {
		if after, ok := strings.CutPrefix(line, "worktree "); ok {
			currentWT = after
		}

		if after, ok := strings.CutPrefix(line, "branch "); ok {
			branch := after

			branch = strings.TrimPrefix(branch, "refs/heads/")
			if currentWT != bareDir {
				worktrees = append(worktrees, struct {
					path   string
					branch string
				}{currentWT, branch})
			}
		}
	}

	hasDefaultWorktree := false

	for _, wt := range worktrees {
		if wt.branch == defaultBranch {
			hasDefaultWorktree = true
			continue
		}

		// Check if remote branch still exists
		_, refErr := gitOutput(bareDir, "rev-parse", "--verify", "refs/remotes/origin/"+wt.branch)
		if refErr == nil {
			continue // remote branch still exists
		}

		// Remote branch deleted — this is a stale worktree
		isDirty := isWorktreeDirty(wt.path)
		result.Findings = append(result.Findings, TidyFinding{
			Type:     "stale_worktree",
			Worktree: filepath.Base(wt.path),
			Branch:   wt.branch,
			Dirty:    isDirty,
		})

		if !opts.Prune {
			continue
		}

		if isDirty {
			result.Actions = append(result.Actions, TidyAction{
				Type:   "skip_dirty",
				Detail: fmt.Sprintf("skipped %s (dirty)", filepath.Base(wt.path)),
			})

			continue
		}

		if err := gitCmdQuiet(bareDir, "worktree", "remove", wt.path); err == nil {
			result.Actions = append(result.Actions, TidyAction{
				Type:   "prune_worktree",
				Detail: "removed " + filepath.Base(wt.path),
			})
			// Also delete the local branch
			_ = gitCmdQuiet(bareDir, "branch", "-D", wt.branch)
		}
	}

	// 3. Check if default branch worktree exists
	if !hasDefaultWorktree {
		result.Findings = append(result.Findings, TidyFinding{
			Type:   "no_default_worktree",
			Branch: defaultBranch,
		})

		if opts.CreateDefault {
			worktreeDir := filepath.Join(repoPath, defaultBranch)
			// Ensure local branch exists tracking remote
			_ = gitCmdQuiet(bareDir, "branch", defaultBranch, "refs/remotes/origin/"+defaultBranch)

			if err := gitCmdQuiet(bareDir, "worktree", "add", worktreeDir, defaultBranch); err == nil {
				_ = gitCmdQuiet(bareDir, "branch", "--set-upstream-to=origin/"+defaultBranch, defaultBranch)
				result.Actions = append(result.Actions, TidyAction{
					Type:   "create_worktree",
					Detail: fmt.Sprintf("created %s/", defaultBranch),
				})
			}
		}
	}

	return result
}

// readLocalHead reads the local HEAD symbolic ref and returns the branch name.
func readLocalHead(bareDir string) string {
	out, err := gitOutput(bareDir, "symbolic-ref", "HEAD")
	if err != nil {
		return ""
	}

	ref := strings.TrimSpace(out)
	if after, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
		return after
	}

	return ""
}

// isWorktreeDirty checks if a worktree has uncommitted changes.
func isWorktreeDirty(wtPath string) bool {
	out, err := gitOutput(wtPath, "status", "--porcelain")
	if err != nil {
		return true // assume dirty on error to be safe
	}

	return strings.TrimSpace(out) != ""
}

func formatTidyFinding(f TidyFinding) string {
	switch f.Type {
	case "head_mismatch":
		return fmt.Sprintf("HEAD → %s (should be %s)", f.Current, f.Expected)
	case "stale_worktree":
		dirty := ""
		if f.Dirty {
			dirty = " [dirty]"
		}

		return fmt.Sprintf("stale worktree %s/ (branch %s deleted from remote)%s", f.Worktree, f.Branch, dirty)
	case "no_default_worktree":
		return "no worktree for default branch " + f.Branch
	default:
		return f.Type
	}
}

func formatTidyAction(a TidyAction) string {
	return a.Detail
}
