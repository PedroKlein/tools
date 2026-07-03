package gitrepo_test

import (
	"testing"

	"github.com/PedroKlein/tools/pkg/gitrepo"
)

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		host    string
		owner   string
		repo    string
		wantErr bool
	}{
		{
			name:  "git@ with .git suffix",
			url:   "git@github.com:PedroKlein/guri.git",
			host:  "github.com",
			owner: "PedroKlein",
			repo:  "guri",
		},
		{
			name:  "git@ without .git suffix",
			url:   "git@github.com:PedroKlein/guri",
			host:  "github.com",
			owner: "PedroKlein",
			repo:  "guri",
		},
		{
			name:  "https with .git suffix",
			url:   "https://git.example.corp/backend/auth-service.git",
			host:  "git.example.corp",
			owner: "backend",
			repo:  "auth-service",
		},
		{
			name:  "https without .git suffix",
			url:   "https://git.example.corp/backend/auth-service",
			host:  "git.example.corp",
			owner: "backend",
			repo:  "auth-service",
		},
		{
			name:  "ssh:// format",
			url:   "ssh://git@git.corp.internal/acme-platform/platform-docs.git",
			host:  "git.corp.internal",
			owner: "acme-platform",
			repo:  "platform-docs",
		},
		{
			name:  "https with port",
			url:   "https://git.example.com:8443/team/project.git",
			host:  "git.example.com",
			owner: "team",
			repo:  "project",
		},
		{
			name:  "ssh:// with port",
			url:   "ssh://git@git.example.com:2222/team/project.git",
			host:  "git.example.com",
			owner: "team",
			repo:  "project",
		},
		{
			name:    "empty string",
			url:     "",
			wantErr: true,
		},
		{
			name:    "no path",
			url:     "https://github.com",
			wantErr: true,
		},
		{
			name:    "single path component",
			url:     "https://github.com/solo",
			wantErr: true,
		},
		{
			name:    "unsupported format",
			url:     "not-a-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, owner, repo, err := gitrepo.ParseRemoteURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got host=%q owner=%q repo=%q", host, owner, repo)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if host != tt.host {
				t.Errorf("host: got %q, want %q", host, tt.host)
			}

			if owner != tt.owner {
				t.Errorf("owner: got %q, want %q", owner, tt.owner)
			}

			if repo != tt.repo {
				t.Errorf("repo: got %q, want %q", repo, tt.repo)
			}
		})
	}
}

func TestSlugFromRemote(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name: "github ssh",
			url:  "git@github.com:PedroKlein/dotfiles.git",
			want: "pedroklein__dotfiles",
		},
		{
			name: "enterprise https",
			url:  "https://git.example.corp/backend/key-manager.git",
			want: "backend__key-manager",
		},
		{
			name: "mixed case normalized",
			url:  "git@github.com:MyOrg/MyRepo.git",
			want: "myorg__myrepo",
		},
		{
			name:    "invalid url",
			url:     "not-a-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gitrepo.SlugFromRemote(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("SlugFromRemote(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestDetectFromCWD(t *testing.T) {
	// This test runs in the tools repo, so it should succeed
	slug, err := gitrepo.DetectFromCWD()
	if err != nil {
		t.Fatalf("DetectFromCWD() error: %v", err)
	}

	if slug == "" {
		t.Error("DetectFromCWD() returned empty slug")
	}
	// We're in the tools repo
	if slug != "pedroklein__tools" {
		t.Errorf("DetectFromCWD() = %q, want pedroklein__tools", slug)
	}
}
