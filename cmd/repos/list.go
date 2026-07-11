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
	Bare     bool   `json:"bare"`
}

// NewRepoEntry creates a RepoEntry from a root and relative path (host/owner/repo).
func NewRepoEntry(root, relPath string) RepoEntry {
	parts := strings.SplitN(relPath, string(filepath.Separator), 3)

	entry := RepoEntry{
		Path:     relPath,
		FullPath: filepath.Join(root, relPath),
		Bare:     isBareRepo(filepath.Join(root, relPath)),
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

// ListRepos walks the root directory looking for repos (.git/ at depth 3).
// Returns paths relative to root (host/owner/repo format).
// If query is non-empty, uses tiered matching:
//  1. Exact repo name match
//  2. Repo name contains query
//  3. owner/repo contains query
//  4. Full path (host/owner/repo) contains query
//
// Returns results from the narrowest non-empty tier.
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

				projectPath := filepath.Join(ownerPath, project.Name())
				if !isGitRepo(projectPath) {
					continue
				}

				repoPath := filepath.Join(host.Name(), owner.Name(), project.Name())
				repos = append(repos, repoPath)
			}
		}
	}

	sort.Strings(repos)

	if query == "" {
		return repos, nil
	}

	return filterReposTiered(repos, query), nil
}

// filterReposTiered applies tiered matching to repos.
// Returns results from the narrowest non-empty tier:
//  1. Exact repo name
//  2. Repo name contains
//  3. owner/repo contains
//  4. Full path contains
func filterReposTiered(repos []string, query string) []string {
	var tier1, tier2, tier3, tier4 []string

	for _, repoPath := range repos {
		parts := strings.SplitN(repoPath, string(filepath.Separator), 3)
		if len(parts) != 3 {
			continue
		}

		repoName := strings.ToLower(parts[2])
		ownerRepo := strings.ToLower(parts[1] + "/" + parts[2])
		fullPath := strings.ToLower(repoPath)

		switch {
		case repoName == query:
			tier1 = append(tier1, repoPath)
		case strings.Contains(repoName, query):
			tier2 = append(tier2, repoPath)
		case strings.Contains(ownerRepo, query):
			tier3 = append(tier3, repoPath)
		case strings.Contains(fullPath, query):
			tier4 = append(tier4, repoPath)
		}
	}

	if len(tier1) > 0 {
		return tier1
	}

	if len(tier2) > 0 {
		return tier2
	}

	if len(tier3) > 0 {
		return tier3
	}

	return tier4
}
