package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneResult is the JSON output for a successful clone.
type CloneResult struct {
	Path     string `json:"path"`
	FullPath string `json:"fullPath"`
	Branch   string `json:"branch"`
	Remote   string `json:"remote"`
}

//nolint:gocognit,gocyclo,funlen // complex CLI handler with many flag combinations and output paths
func runClone(args []string) {
	if len(args) == 0 {
		if jsonOutput {
			writeJSONError("usage: repos clone <url> [-b branch] [--flat]", ExitError)
		}

		fmt.Fprintln(os.Stderr, "usage: repos clone <url> [-b branch] [--flat]")
		os.Exit(ExitError)
	}

	var url, branch string

	quiet := false
	flat := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-b", "--branch":
			if i+1 >= len(args) {
				if jsonOutput {
					writeJSONError("-b requires a branch name", ExitError)
				}

				fmt.Fprintln(os.Stderr, "error: -b requires a branch name")
				os.Exit(ExitError)
			}

			i++
			branch = args[i]
		case "-q", "--quiet":
			quiet = true
		case "--flat":
			flat = true
		default:
			if url == "" {
				url = args[i]
			} else {
				if jsonOutput {
					writeJSONError("unexpected argument: "+args[i], ExitError)
				}

				fmt.Fprintf(os.Stderr, "error: unexpected argument: %s\n", args[i])
				os.Exit(ExitError)
			}
		}
	}

	if url == "" {
		if jsonOutput {
			writeJSONError("usage: repos clone <url> [-b branch] [--flat]", ExitError)
		}

		fmt.Fprintln(os.Stderr, "usage: repos clone <url> [-b branch] [--flat]")
		os.Exit(ExitError)
	}

	host, owner, repo, err := ParseRemoteURL(url)
	if err != nil {
		if jsonOutput {
			writeJSONError(fmt.Sprintf("error parsing URL: %v", err), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error parsing URL: %v\n", err)
		os.Exit(ExitError)
	}

	root := ReposRoot()
	repoDir := filepath.Join(root, host, owner, repo)
	bareDir := filepath.Join(repoDir, ".git")

	// Check if already exists
	if _, err := os.Stat(bareDir); err == nil {
		if jsonOutput {
			writeJSONError("already exists: "+repoDir, ExitError)
		}

		fmt.Fprintf(os.Stderr, "already exists: %s\n", repoDir)
		os.Exit(ExitError)
	}

	// Create parent directories
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
		if jsonOutput {
			writeJSONError(fmt.Sprintf("error creating directory: %v", err), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error creating directory: %v\n", err)
		os.Exit(ExitError)
	}

	if !quiet && !jsonOutput {
		fmt.Printf("Cloning %s into %s...\n", url, repoDir)
	}

	if flat {
		runFlatClone(repoDir, url, branch, host, owner, repo, quiet)
		return
	}

	// Clone bare
	var cloneErr error
	if quiet || jsonOutput {
		cloneErr = gitCmdQuiet(repoDir, "clone", "--bare", url, ".git")
	} else {
		cloneErr = gitCmd(repoDir, "clone", "--bare", url, ".git")
	}

	if cloneErr != nil {
		if removeErr := os.RemoveAll(repoDir); removeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: cleanup failed: %v\n", removeErr)
		}

		if jsonOutput {
			writeJSONError(fmt.Sprintf("error cloning: %v", cloneErr), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error cloning: %v\n", cloneErr)
		os.Exit(ExitError)
	}

	// Configure fetch refspec
	_ = gitCmdQuiet(bareDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")

	// Fetch all remote branches
	_ = gitCmdQuiet(bareDir, "fetch", "origin")

	// Determine which branch to create worktree for
	worktreeBranch := detectDefaultBranch(bareDir)
	if branch != "" {
		worktreeBranch = branch
	}

	// Create worktree via the shared cascade helper.
	worktreeDir := filepath.Join(repoDir, worktreeBranch)
	if err := addDefaultWorktree(bareDir, worktreeDir, worktreeBranch); err != nil {
		msg := fmt.Sprintf("bare repo cloned to %s but worktree add for %q failed: %v. Retry with: repos wt add %s", bareDir, worktreeBranch, err, worktreeBranch)

		if jsonOutput {
			writeJSONError(msg, ExitError)
		}

		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
		os.Exit(ExitError)
	}

	// Set up tracking (best-effort; branch may already track after --track, and
	// an orphan branch has no remote yet).
	_ = gitCmdQuiet(bareDir, "branch", "--set-upstream-to=origin/"+worktreeBranch, worktreeBranch)

	relPath := filepath.Join(host, owner, repo)

	if jsonOutput {
		writeJSON(CloneResult{
			Path:     relPath,
			FullPath: repoDir,
			Branch:   worktreeBranch,
			Remote:   url,
		})

		return
	}

	if quiet {
		// Just print the worktree path for scripting
		fmt.Println(worktreeDir)
		return
	}

	fmt.Printf("\n%s/%s/%s\n  ├── .git/\n  └── %s/\n", host, owner, repo, worktreeBranch)

	// Run post-clone hooks
	runHooksForEvent("post-clone", []HookRepoInfo{{
		Path:   repoDir,
		Name:   repo,
		Host:   host,
		Owner:  owner,
		Branch: worktreeBranch,
	}})
}

// runFlatClone performs a regular (non-bare) git clone into the canonical path.
func runFlatClone(repoDir, url, branch, host, owner, repo string, quiet bool) {
	cloneArgs := []string{"clone", url, "."}
	if branch != "" {
		cloneArgs = []string{"clone", "--branch", branch, url, "."}
	}

	var cloneErr error
	if quiet || jsonOutput {
		cloneErr = gitCmdQuiet(repoDir, cloneArgs...)
	} else {
		cloneErr = gitCmd(repoDir, cloneArgs...)
	}

	if cloneErr != nil {
		if removeErr := os.RemoveAll(repoDir); removeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: cleanup failed: %v\n", removeErr)
		}

		if jsonOutput {
			writeJSONError(fmt.Sprintf("error cloning: %v", cloneErr), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error cloning: %v\n", cloneErr)
		os.Exit(ExitError)
	}

	// Detect the checked-out branch
	checkedOut := branch
	if checkedOut == "" {
		if out, err := gitOutput(repoDir, "symbolic-ref", "--short", "HEAD"); err == nil {
			checkedOut = strings.TrimSpace(out)
		} else {
			checkedOut = "main"
		}
	}

	relPath := filepath.Join(host, owner, repo)

	if jsonOutput {
		writeJSON(CloneResult{
			Path:     relPath,
			FullPath: repoDir,
			Branch:   checkedOut,
			Remote:   url,
		})

		return
	}

	if quiet {
		fmt.Println(repoDir)
		return
	}

	fmt.Printf("\n%s/%s/%s (flat)\n", host, owner, repo)

	// Run post-clone hooks
	runHooksForEvent("post-clone", []HookRepoInfo{{
		Path:   repoDir,
		Name:   repo,
		Host:   host,
		Owner:  owner,
		Branch: checkedOut,
	}})
}

// addDefaultWorktree creates the initial worktree for a freshly-cloned bare
// repository, trying three strategies in cascade:
//
//  1. Plain `worktree add <dir> <branch>` — works when the local branch
//     already exists (the normal case after a bare clone with commits).
//  2. `worktree add --track -b <branch> <dir> origin/<branch>` — creates a
//     new local tracking branch when only the remote ref exists.
//  3. `worktree add --orphan -b <branch> <dir>` — for empty repositories
//     that have no commits on any branch; produces a worktree ready to
//     receive the first commit. Requires git 2.42+.
//
// Returns a wrapped error carrying every attempt's captured stderr when all
// three fail.
func addDefaultWorktree(bareDir, worktreeDir, branch string) error {
	err := gitCmdQuiet(bareDir, "worktree", "add", worktreeDir, branch)
	if err == nil {
		return nil
	}

	trackErr := gitCmdQuiet(bareDir, "worktree", "add", "--track", "-b", branch, worktreeDir, "origin/"+branch)
	if trackErr == nil {
		return nil
	}

	orphanErr := gitCmdQuiet(bareDir, "worktree", "add", "--orphan", "-b", branch, worktreeDir)
	if orphanErr == nil {
		return nil
	}

	return fmt.Errorf("%w (track: %w; orphan: %w)", err, trackErr, orphanErr)
}

// fallbackDefaultBranch is the branch name assumed when origin/HEAD cannot be
// determined and neither 'main' nor 'master' exists on the remote.
const fallbackDefaultBranch = "main"

// detectDefaultBranch determines the default branch name for a bare repo.
// It uses origin/HEAD as the source of truth — this reflects the remote's
// declared default and is not polluted by local worktree creation (unlike HEAD).
func detectDefaultBranch(bareDir string) string {
	// Prefer origin/HEAD: set by the remote and unaffected by worktree ops.
	out, err := gitOutput(bareDir, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		ref := strings.TrimSpace(out)
		if after, ok := strings.CutPrefix(ref, "refs/remotes/origin/"); ok {
			return after
		}
	}

	// Fallback: check if main or master exists on the remote.
	for _, candidate := range []string{fallbackDefaultBranch, "master"} {
		if err := gitCmdQuiet(bareDir, "rev-parse", "--verify", "refs/remotes/origin/"+candidate); err == nil {
			return candidate
		}
	}

	return fallbackDefaultBranch
}

// gitCmd runs a git command in the given directory, inheriting stdout/stderr.
func gitCmd(dir string, args ...string) error {
	cmd := gitCommand(dir, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %v: %w", args, err)
	}

	return nil
}

// gitOutput runs a git command and returns stdout.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := gitCommand(dir, args...)
	out, err := cmd.Output()

	return string(out), err
}

// gitCommand creates an exec.Cmd for git in the given directory.
func gitCommand(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	return cmd
}
