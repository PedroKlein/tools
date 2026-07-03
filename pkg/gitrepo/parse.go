// Package gitrepo provides utilities for parsing git remote URLs and detecting
// the current repository from the working directory.
package gitrepo

import (
	"errors"
	"fmt"
	"strings"
)

// ParseRemoteURL parses a git remote URL into host, owner, and repo components.
// Supported formats:
//   - git@host:owner/repo.git
//   - https://host/owner/repo.git
//   - ssh://git@host/owner/repo.git
//   - ssh://git@host:port/owner/repo.git
//   - https://host:port/owner/repo.git
func ParseRemoteURL(url string) (host, owner, repo string, err error) {
	if url == "" {
		return "", "", "", errors.New("empty URL")
	}

	// Normalize: strip trailing .git
	url = strings.TrimSuffix(url, ".git")

	switch {
	case strings.HasPrefix(url, "git@"):
		return parseSCP(url)
	case strings.HasPrefix(url, "ssh://"):
		return parseSSH(url)
	case strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://"):
		return parseHTTPS(url)
	default:
		return "", "", "", fmt.Errorf("unsupported URL format: %s", url)
	}
}

func parseSCP(url string) (host, owner, repo string, err error) {
	rest := strings.TrimPrefix(url, "git@")

	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", "", fmt.Errorf("invalid SCP URL: %s", url)
	}

	host = parts[0]
	path := parts[1]

	owner, repo, err = splitOwnerRepo(path)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid SCP URL: %w", err)
	}

	return host, owner, repo, nil
}

func parseSSH(url string) (host, owner, repo string, err error) {
	rest := strings.TrimPrefix(url, "ssh://")

	if idx := strings.Index(rest, "@"); idx != -1 {
		rest = rest[idx+1:]
	}

	before, after, ok := strings.Cut(rest, "/")
	if !ok {
		return "", "", "", errors.New("invalid SSH URL: no path")
	}

	hostPort := before
	path := after

	host = stripPort(hostPort)

	owner, repo, err = splitOwnerRepo(path)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid SSH URL: %w", err)
	}

	return host, owner, repo, nil
}

func parseHTTPS(url string) (host, owner, repo string, err error) {
	rest := url
	if idx := strings.Index(rest, "://"); idx != -1 {
		rest = rest[idx+3:]
	}

	before, after, ok := strings.Cut(rest, "/")
	if !ok {
		return "", "", "", errors.New("invalid HTTPS URL: no path")
	}

	hostPort := before
	path := after

	host = stripPort(hostPort)

	owner, repo, err = splitOwnerRepo(path)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid HTTPS URL: %w", err)
	}

	return host, owner, repo, nil
}

func stripPort(hostPort string) string {
	if idx := strings.LastIndex(hostPort, ":"); idx != -1 {
		return hostPort[:idx]
	}

	return hostPort
}

func splitOwnerRepo(path string) (owner, repo string, err error) {
	path = strings.TrimPrefix(path, "/")

	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected owner/repo, got %q", path)
	}

	return parts[0], parts[1], nil
}
