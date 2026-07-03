package main

import "testing"

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
			name:  "ssh:// without .git",
			url:   "ssh://git@git.corp.internal/acme-platform/platform-docs",
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
			name:    "no owner/repo path",
			url:     "https://github.com",
			wantErr: true,
		},
		{
			name:    "only one path component",
			url:     "https://github.com/solo",
			wantErr: true,
		},
		{
			name:    "random string",
			url:     "not-a-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, owner, repo, err := ParseRemoteURL(tt.url)
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
