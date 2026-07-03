package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RepoEntry is the JSON representation of a listed repo.
type RepoEntry struct {
	Path     string `json:"path"`
	FullPath string `json:"fullPath"`
	Host     string `json:"host"`
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
}

// NewRepoEntry creates a RepoEntry from a root and relative path (host/owner/repo).
func NewRepoEntry(root, relPath string) RepoEntry {
	parts := strings.SplitN(relPath, string(filepath.Separator), 3)

	entry := RepoEntry{
		Path:     relPath,
		FullPath: filepath.Join(root, relPath),
	}
	if len(parts) >= 1 {
		entry.Host = parts[0]
	}

	if len(parts) >= 2 {
		entry.Owner = parts[1]
	}

	if len(parts) >= 3 {
		entry.Repo = parts[2]
	}

	return entry
}

// ListRepos walks the root directory looking for repos (.git/ bare repo at depth 3).
// Returns paths relative to root (host/owner/repo format).
// If query is non-empty, filters by case-insensitive substring match.
//
//nolint:gocognit,gocyclo // deeply nested directory walk at three levels (host/owner/repo)
func ListRepos(root, query string) ([]string, error) {
	var repos []string

	query = strings.ToLower(query)

	// Walk at exactly 3 levels deep: host/owner/repo
	hosts, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading root %s: %w", root, err)
	}

	for _, host := range hosts {
		if !host.IsDir() || strings.HasPrefix(host.Name(), ".") {
			continue
		}

		hostPath := filepath.Join(root, host.Name())

		owners, err := os.ReadDir(hostPath)
		if err != nil {
			continue
		}

		for _, owner := range owners {
			if !owner.IsDir() || strings.HasPrefix(owner.Name(), ".") {
				continue
			}

			ownerPath := filepath.Join(hostPath, owner.Name())

			projects, err := os.ReadDir(ownerPath)
			if err != nil {
				continue
			}

			for _, project := range projects {
				if !project.IsDir() || strings.HasPrefix(project.Name(), ".") {
					continue
				}

				// Check for bare git repo (.git/ directory with bare = true)
				projectPath := filepath.Join(ownerPath, project.Name())
				if !isBareRepo(projectPath) {
					continue
				}

				repoPath := filepath.Join(host.Name(), owner.Name(), project.Name())

				if query != "" && !strings.Contains(strings.ToLower(repoPath), query) {
					continue
				}

				repos = append(repos, repoPath)
			}
		}
	}

	sort.Strings(repos)

	return repos, nil
}
