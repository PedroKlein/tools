package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func runBrowse(args []string) {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	repoPath, relPath, err := resolveRepo(query)
	if err != nil {
		if jsonOutput {
			writeJSONError(err.Error(), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(ExitError)
	}

	bareDir := filepath.Join(repoPath, ".git")

	remoteURL, err := gitOutput(bareDir, "remote", "get-url", "origin")
	if err != nil {
		if jsonOutput {
			writeJSONError(fmt.Sprintf("cannot get remote URL: %v", err), ExitError)
		}

		fmt.Fprintf(os.Stderr, "error: cannot get remote URL: %v\n", err)
		os.Exit(ExitError)
	}

	remoteURL = strings.TrimSpace(remoteURL)

	browserURL := remoteToBrowserURL(remoteURL)

	if jsonOutput {
		writeJSON(struct {
			URL  string `json:"url"`
			Repo string `json:"repo"`
		}{URL: browserURL, Repo: relPath})

		return
	}

	if err := openBrowser(browserURL); err != nil {
		fmt.Fprintf(os.Stderr, "error opening browser: %v\n", err)
		os.Exit(ExitError)
	}

	fmt.Println(browserURL)
}

// remoteToBrowserURL converts a git remote URL to an HTTPS browser URL.
// Handles:
//
//	git@github.com:owner/repo.git       → https://github.com/owner/repo
//	https://github.com/owner/repo.git   → https://github.com/owner/repo
//	ssh://git@github.com/owner/repo.git → https://github.com/owner/repo
func remoteToBrowserURL(remote string) string {
	// Strip .git suffix
	remote = strings.TrimSuffix(remote, ".git")

	// Already https/http — return as-is
	if strings.HasPrefix(remote, "https://") || strings.HasPrefix(remote, "http://") {
		return remote
	}

	// Handle ssh:// scheme: ssh://[user@]host[:port]/owner/repo
	if after, ok := strings.CutPrefix(remote, "ssh://"); ok {
		remote = after
		// Strip user@ prefix
		if at := strings.Index(remote, "@"); at >= 0 {
			remote = remote[at+1:]
		}
		// Strip port from host (host:port/path → host/path)
		if colon := strings.Index(remote, ":"); colon >= 0 {
			if slash := strings.Index(remote, "/"); slash > colon {
				remote = remote[:colon] + remote[slash:]
			}
		}

		return "https://" + remote
	}

	// SCP format: git@host:owner/repo  (or user@host:owner/repo)
	if at := strings.Index(remote, "@"); at >= 0 {
		if colon := strings.Index(remote, ":"); colon > at {
			host := remote[at+1 : colon]
			path := remote[colon+1:]

			return "https://" + host + "/" + path
		}
	}

	// Unknown format; return unchanged.
	return remote
}

// openBrowser opens a URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("opening browser: %w", err)
	}

	return nil
}
