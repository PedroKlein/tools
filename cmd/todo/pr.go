package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// PrURL holds parsed components of a GitHub PR URL.
type PrURL struct {
	Host   string
	Owner  string
	Repo   string
	Number int
	URL    string
}

var prURLRegex = regexp.MustCompile(`^https?://([^/]+)/([^/]+)/([^/]+)/pull/(\d+)/?`)

// ParsePrURL detects if text is a GitHub PR URL and extracts components.
// Returns nil if text is not a PR URL.
func ParsePrURL(text string) *PrURL {
	text = strings.TrimSpace(text)

	matches := prURLRegex.FindStringSubmatch(text)
	if matches == nil {
		return nil
	}

	number, err := strconv.Atoi(matches[4])
	if err != nil {
		return nil
	}

	return &PrURL{
		Host:   matches[1],
		Owner:  matches[2],
		Repo:   matches[3],
		Number: number,
		URL:    strings.TrimSuffix(text, "/"),
	}
}

// FetchPrMeta fetches PR metadata using the gh CLI.
// Falls back to basic metadata from the URL if gh is unavailable or fails.
func FetchPrMeta(pr *PrURL) PrMeta {
	meta, err := fetchPrMetaGH(pr)
	if err != nil {
		return fallbackPrMeta(pr)
	}

	return meta
}

func fetchPrMetaGH(pr *PrURL) (PrMeta, error) {
	// Check if gh is available
	if _, err := exec.LookPath("gh"); err != nil {
		return PrMeta{}, fmt.Errorf("gh not found: %w", err)
	}

	args := []string{
		"pr", "view",
		strconv.Itoa(pr.Number),
		"--repo", fmt.Sprintf("%s/%s", pr.Owner, pr.Repo),
		"--json", "title,author,state,headRefName",
	}

	// Add --hostname for enterprise GitHub instances
	if pr.Host != "github.com" {
		args = append(args, "--hostname", pr.Host)
	}

	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		return PrMeta{}, fmt.Errorf("gh pr view: %w", err)
	}

	var raw struct {
		Title  string `json:"title"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		State       string `json:"state"`
		HeadRefName string `json:"headRefName"`
	}

	if err := json.Unmarshal(out, &raw); err != nil {
		return PrMeta{}, fmt.Errorf("parsing gh output: %w", err)
	}

	return PrMeta{
		Title:  raw.Title,
		Author: raw.Author.Login,
		State:  strings.ToLower(raw.State),
		Branch: raw.HeadRefName,
		Host:   pr.Host,
		Owner:  pr.Owner,
		Repo:   pr.Repo,
		Number: pr.Number,
	}, nil
}

func fallbackPrMeta(pr *PrURL) PrMeta {
	return PrMeta{
		Title:  fmt.Sprintf("%s/%s #%d", pr.Owner, pr.Repo, pr.Number),
		Author: "unknown",
		State:  "unknown",
		Branch: "unknown",
		Host:   pr.Host,
		Owner:  pr.Owner,
		Repo:   pr.Repo,
		Number: pr.Number,
	}
}

// CreateReviewTask creates a review task from a PR URL, fetching metadata.
func CreateReviewTask(pr *PrURL, store *Store) (Task, error) {
	meta := FetchPrMeta(pr)

	title := fmt.Sprintf("Review: %s (%s/%s #%d)", meta.Title, pr.Owner, pr.Repo, pr.Number)

	nextID, err := store.NextID(ReviewRepoID)
	if err != nil {
		return Task{}, fmt.Errorf("getting next ID: %w", err)
	}

	task := NewTask(nextID, title, ReviewRepoID, TypeReview)
	task.URL = pr.URL
	task.PrMeta = &meta
	task.Description = fmt.Sprintf("PR by %s on branch %s. State: %s.", meta.Author, meta.Branch, meta.State)

	// Load reviews, append, save
	tasks, err := store.LoadScope(ReviewRepoID)
	if err != nil {
		return Task{}, fmt.Errorf("loading reviews: %w", err)
	}

	tasks = append(tasks, task)
	if err := store.SaveScope(ReviewRepoID, tasks); err != nil {
		return Task{}, fmt.Errorf("saving review: %w", err)
	}

	return task, nil
}
