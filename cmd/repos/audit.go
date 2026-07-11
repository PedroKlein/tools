package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AuditResult is the JSON representation of a repo's audit status.
type AuditResult struct {
	Path     string    `json:"path"`
	Status   string    `json:"status"` // "clean" or "dirty"
	Findings []Finding `json:"findings"`
}

// Finding represents a single audit issue.
type Finding struct {
	Type     string `json:"type"`               // "dirty_worktree", "unpushed", "no_upstream"
	Worktree string `json:"worktree,omitempty"` // for dirty_worktree
	Files    int    `json:"files,omitempty"`    // for dirty_worktree
	Branch   string `json:"branch,omitempty"`   // for unpushed, no_upstream
	Track    string `json:"track,omitempty"`    // for unpushed
}

func runAudit(args []string) {
	query := ""
	if len(args) > 0 {
		query = args[0]
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

	results := make([]AuditResult, 0, len(repos))

	dirty := 0

	for _, repo := range repos {
		repoPath := filepath.Join(root, repo)
		findings := auditRepoStructured(repoPath)

		status := "clean"
		if len(findings) > 0 {
			status = "dirty"
			dirty++
		}

		results = append(results, AuditResult{
			Path:     repo,
			Status:   status,
			Findings: findings,
		})
	}

	if jsonOutput {
		writeJSON(results)

		if dirty > 0 {
			os.Exit(ExitError)
		}

		return
	}

	// Human-readable output
	for _, r := range results {
		if r.Status == "dirty" {
			fmt.Printf("⚠ %s\n", r.Path)

			for _, f := range r.Findings {
				fmt.Printf("  %s\n", formatFinding(f))
			}
		}
	}

	if dirty == 0 {
		fmt.Printf("✓ All %d repos clean\n", len(repos))
	} else {
		fmt.Printf("\n%d/%d repos have issues\n", dirty, len(repos))
		os.Exit(ExitError)
	}
}

// formatFinding renders a Finding as a human-readable string.
func formatFinding(f Finding) string {
	switch f.Type {
	case "dirty_worktree":
		return fmt.Sprintf("dirty worktree %s/ (%d files)", f.Worktree, f.Files)
	case "unpushed":
		return fmt.Sprintf("branch %s is %s", f.Branch, f.Track)
	case "no_upstream":
		return fmt.Sprintf("branch %s has no upstream (local only)", f.Branch)
	default:
		return f.Type
	}
}

// auditRepoStructured checks a single repo and returns structured findings.
func auditRepoStructured(repoPath string) []Finding {
	var findings []Finding

	bareDir := filepath.Join(repoPath, ".git")

	// For non-bare repos, check dirty status on repo root directly
	if !isBareRepo(repoPath) {
		statusOut, err := gitOutput(repoPath, "status", "--porcelain")
		if err == nil && strings.TrimSpace(statusOut) != "" {
			lines := strings.Split(strings.TrimSpace(statusOut), "\n")
			findings = append(findings, Finding{
				Type:     "dirty_worktree",
				Worktree: ".",
				Files:    len(lines),
			})
		}

		findings = append(findings, auditBranches(repoPath)...)

		return findings
	}

	// List worktrees
	out, err := gitOutput(bareDir, "worktree", "list", "--porcelain")
	if err != nil {
		findings = append(findings, Finding{Type: "error", Track: fmt.Sprintf("cannot list worktrees: %v", err)})
		return findings
	}

	// Check each worktree for dirty status
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}

		wtPath := strings.TrimPrefix(line, "worktree ")
		if wtPath == bareDir {
			continue // skip bare repo itself
		}

		// Check for uncommitted changes
		statusOut, statusErr := gitOutput(wtPath, "status", "--porcelain")
		if statusErr != nil {
			continue
		}

		statusOut = strings.TrimSpace(statusOut)
		if statusOut != "" {
			lines := strings.Split(statusOut, "\n")
			findings = append(findings, Finding{
				Type:     "dirty_worktree",
				Worktree: filepath.Base(wtPath),
				Files:    len(lines),
			})
		}
	}

	findings = append(findings, auditBranches(bareDir)...)

	return findings
}

// auditBranches checks for unpushed or untracked branches in the given git dir.
func auditBranches(gitDir string) []Finding {
	var findings []Finding

	branchOut, err := gitOutput(gitDir, "for-each-ref", "--format=%(refname:short) %(upstream:track)", "refs/heads/")
	if err != nil {
		return findings
	}

	for line := range strings.SplitSeq(strings.TrimSpace(branchOut), "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		branch := parts[0]

		track := ""
		if len(parts) > 1 {
			track = parts[1]
		}

		if strings.Contains(track, "ahead") {
			findings = append(findings, Finding{
				Type:   "unpushed",
				Branch: branch,
				Track:  track,
			})
		}

		// Check if branch has no upstream at all
		upOut, _ := gitOutput(gitDir, "config", "branch."+branch+".remote")
		if strings.TrimSpace(upOut) == "" {
			// Check if a matching remote ref exists
			_, refErr := gitOutput(gitDir, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+branch)
			if refErr == nil {
				continue
			}

			findings = append(findings, Finding{
				Type:   "no_upstream",
				Branch: branch,
			})
		}
	}

	return findings
}
