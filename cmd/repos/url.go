package main

import (
	"fmt"

	"github.com/PedroKlein/tools/pkg/gitrepo"
)

// ParseRemoteURL parses a git remote URL into host, owner, and repo components.
// Delegates to the shared gitrepo package.
func ParseRemoteURL(url string) (host, owner, repo string, err error) {
	h, o, r, parseErr := gitrepo.ParseRemoteURL(url)
	if parseErr != nil {
		return "", "", "", fmt.Errorf("parsing remote URL: %w", parseErr)
	}

	return h, o, r, nil
}
