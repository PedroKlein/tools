package main

import (
	"testing"
)

func TestParseGlobalFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantJSON bool
		wantArgs []string
	}{
		{
			name:     "no flags",
			args:     []string{"repos", "list"},
			wantJSON: false,
			wantArgs: []string{"repos", "list"},
		},
		{
			name:     "--json before subcommand",
			args:     []string{"repos", "--json", "list"},
			wantJSON: true,
			wantArgs: []string{"repos", "list"},
		},
		{
			name:     "--json after subcommand",
			args:     []string{"repos", "list", "--json"},
			wantJSON: true,
			wantArgs: []string{"repos", "list"},
		},
		{
			name:     "--json with query",
			args:     []string{"repos", "list", "--json", "auth"},
			wantJSON: true,
			wantArgs: []string{"repos", "list", "auth"},
		},
		{
			name:     "--json at end after query",
			args:     []string{"repos", "path", "guri", "--json"},
			wantJSON: true,
			wantArgs: []string{"repos", "path", "guri"},
		},
		{
			name:     "no json with other flags",
			args:     []string{"repos", "list", "-p"},
			wantJSON: false,
			wantArgs: []string{"repos", "list", "-p"},
		},
		{
			name:     "--json with other flags",
			args:     []string{"repos", "clone", "--json", "-b", "main", "url"},
			wantJSON: true,
			wantArgs: []string{"repos", "clone", "-b", "main", "url"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotJSON, gotArgs := parseGlobalFlags(tt.args)
			if gotJSON != tt.wantJSON {
				t.Errorf("parseGlobalFlags() json = %v, want %v", gotJSON, tt.wantJSON)
			}

			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("parseGlobalFlags() args = %v, want %v", gotArgs, tt.wantArgs)
				return
			}

			for i := range gotArgs {
				if gotArgs[i] != tt.wantArgs[i] {
					t.Errorf("parseGlobalFlags() args[%d] = %q, want %q", i, gotArgs[i], tt.wantArgs[i])
				}
			}
		})
	}
}
