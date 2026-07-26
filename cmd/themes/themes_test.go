package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGlobalFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantJSON bool
		wantLen  int
	}{
		{"no flag", []string{"theme", "list"}, false, 2},
		{"json before cmd", []string{"theme", "--json", "list"}, true, 2},
		{"json after cmd", []string{"theme", "list", "--json"}, true, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotJSON, cleaned := parseGlobalFlags(tt.args)
			if gotJSON != tt.wantJSON {
				t.Errorf("json=%v want %v", gotJSON, tt.wantJSON)
			}
			if len(cleaned) != tt.wantLen {
				t.Errorf("cleaned len=%d want %d (%v)", len(cleaned), tt.wantLen, cleaned)
			}
		})
	}
}

// v3 state-file tests (TestStateRoundTrip, TestLoadStateMissingFile,
// TestSaveAtomicUsesRename) were deleted with cmd/themes/state.go in a3.
// State lives at $XDG_STATE_HOME/themes/state.json and is covered by
// internal/state/state_test.go and stress_test.go.

func TestSwapSymlinkAtomic(t *testing.T) {
	// Simulate atomic symlink swap: link -> A, then swap to B.
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "themes", "A"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "themes", "B"), 0o755)
	link := filepath.Join(dir, "themes", ".current")
	if err := swapSymlink(link, "A"); err != nil {
		t.Fatalf("first swap: %v", err)
	}
	tgt, _ := os.Readlink(link)
	if tgt != "A" {
		t.Fatalf("after first swap: %s", tgt)
	}
	if err := swapSymlink(link, "B"); err != nil {
		t.Fatalf("second swap: %v", err)
	}
	tgt, _ = os.Readlink(link)
	if tgt != "B" {
		t.Fatalf("after second swap: %s", tgt)
	}
}

func TestParseAlacrittyForSwatch(t *testing.T) {
	t.Skip("v3 alacritty.toml parser deleted in P1.11; superseded by palette.Load")
}
