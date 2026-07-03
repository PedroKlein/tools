package gitrepo

import (
	"strings"
)

// SlugFromRemote parses a git remote URL and returns "owner__repo" slug.
// Uses __ as separator since GitHub doesn't allow it in owner/repo names,
// making it unambiguous for repos with dashes (e.g., start__key-manager).
func SlugFromRemote(remoteURL string) (string, error) {
	_, owner, repo, err := ParseRemoteURL(remoteURL)
	if err != nil {
		return "", err
	}

	slug := strings.ToLower(owner + "__" + repo)

	return slug, nil
}
