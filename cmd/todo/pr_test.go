package main

import (
	"testing"
)

func TestParsePrURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *PrURL
	}{
		{
			name:  "github.com standard",
			input: "https://github.com/PedroKlein/guri/pull/42",
			want:  &PrURL{Host: "github.com", Owner: "PedroKlein", Repo: "guri", Number: 42, URL: "https://github.com/PedroKlein/guri/pull/42"},
		},
		{
			name:  "github.com with trailing slash",
			input: "https://github.com/PedroKlein/guri/pull/42/",
			want:  &PrURL{Host: "github.com", Owner: "PedroKlein", Repo: "guri", Number: 42, URL: "https://github.com/PedroKlein/guri/pull/42"},
		},
		{
			name:  "enterprise host",
			input: "https://git.example.corp/backend/key-manager/pull/1138",
			want:  &PrURL{Host: "git.example.corp", Owner: "backend", Repo: "key-manager", Number: 1138, URL: "https://git.example.corp/backend/key-manager/pull/1138"},
		},
		{
			name:  "with whitespace",
			input: "  https://github.com/org/repo/pull/7  ",
			want:  &PrURL{Host: "github.com", Owner: "org", Repo: "repo", Number: 7, URL: "https://github.com/org/repo/pull/7"},
		},
		{
			name:  "not a PR URL - issues",
			input: "https://github.com/org/repo/issues/42",
			want:  nil,
		},
		{
			name:  "not a PR URL - random",
			input: "fix the rendering bug",
			want:  nil,
		},
		{
			name:  "not a PR URL - empty",
			input: "",
			want:  nil,
		},
		{
			name:  "not a PR URL - repo only",
			input: "https://github.com/org/repo",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePrURL(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}

				return
			}

			if got == nil {
				t.Fatal("expected non-nil result")
			}

			if got.Host != tt.want.Host {
				t.Errorf("Host = %q, want %q", got.Host, tt.want.Host)
			}

			if got.Owner != tt.want.Owner {
				t.Errorf("Owner = %q, want %q", got.Owner, tt.want.Owner)
			}

			if got.Repo != tt.want.Repo {
				t.Errorf("Repo = %q, want %q", got.Repo, tt.want.Repo)
			}

			if got.Number != tt.want.Number {
				t.Errorf("Number = %d, want %d", got.Number, tt.want.Number)
			}

			if got.URL != tt.want.URL {
				t.Errorf("URL = %q, want %q", got.URL, tt.want.URL)
			}
		})
	}
}

func TestFetchPrMetaFallback(t *testing.T) {
	// FetchPrMeta should gracefully fall back when gh is unavailable or fails.
	// In a test environment without proper gh auth, it should return fallback meta.
	pr := &PrURL{
		Host:   "github.com",
		Owner:  "test-org",
		Repo:   "test-repo",
		Number: 99,
		URL:    "https://github.com/test-org/test-repo/pull/99",
	}

	meta := FetchPrMeta(pr)
	// Should at least have the basic fields populated
	if meta.Host != "github.com" {
		t.Errorf("Host = %q, want github.com", meta.Host)
	}

	if meta.Owner != "test-org" {
		t.Errorf("Owner = %q, want test-org", meta.Owner)
	}

	if meta.Repo != "test-repo" {
		t.Errorf("Repo = %q, want test-repo", meta.Repo)
	}

	if meta.Number != 99 {
		t.Errorf("Number = %d, want 99", meta.Number)
	}
}

func TestCreateReviewTask(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	pr := &PrURL{
		Host:   "github.com",
		Owner:  "org",
		Repo:   "repo",
		Number: 42,
		URL:    "https://github.com/org/repo/pull/42",
	}

	task, err := CreateReviewTask(pr, store)
	if err != nil {
		t.Fatalf("CreateReviewTask: %v", err)
	}

	if task.Type != TypeReview {
		t.Errorf("Type = %q, want review", task.Type)
	}

	if task.RepoID != ReviewRepoID {
		t.Errorf("RepoID = %q, want %q", task.RepoID, ReviewRepoID)
	}

	if task.URL != pr.URL {
		t.Errorf("URL = %q, want %q", task.URL, pr.URL)
	}

	if task.PrMeta == nil {
		t.Fatal("PrMeta should not be nil")
	}

	if task.PrMeta.Number != 42 {
		t.Errorf("PrMeta.Number = %d, want 42", task.PrMeta.Number)
	}

	// Verify it was persisted
	tasks, err := store.LoadScope(ReviewRepoID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("expected 1 review task, got %d", len(tasks))
	}
}
