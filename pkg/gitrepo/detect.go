package gitrepo

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// DetectFromCWD determines the repository slug from the current working directory
// by reading the git remote origin URL. Returns "owner-repo" slug.
// Returns an error if not inside a git repository or no origin remote is configured.
func DetectFromCWD() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("detecting git remote: %w", err)
	}

	remote := strings.TrimSpace(string(out))
	if remote == "" {
		return "", errors.New("empty git remote URL")
	}

	return SlugFromRemote(remote)
}
