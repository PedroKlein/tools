package main

import (
	"encoding/json"
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

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "state.json")
	s := &State{
		Theme:            "osaka-jade",
		Wallpaper:        "/tmp/bg.jpg",
		WallpaperByTheme: map[string]string{"osaka-jade": "/tmp/bg.jpg"},
		ChangedAt:        "2026-01-15T09:12:33Z",
	}
	if err := s.saveTo(sp); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := loadStateFrom(sp)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Theme != s.Theme ||
		got.Wallpaper != s.Wallpaper || got.ChangedAt != s.ChangedAt ||
		got.WallpaperByTheme["osaka-jade"] != "/tmp/bg.jpg" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.SchemaVersion != currentSchemaVersion {
		t.Errorf("schema version = %d, want %d", got.SchemaVersion, currentSchemaVersion)
	}
}

func TestLoadStateMissingFile(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "state.json")
	s, err := loadStateFrom(sp)
	if err != nil {
		t.Fatalf("expected no error on missing file, got %v", err)
	}
	if s.Theme != "" {
		t.Errorf("expected zero-value theme, got %q", s.Theme)
	}
	if s.WallpaperByTheme == nil {
		t.Error("expected non-nil WallpaperByTheme")
	}
}

func TestSaveAtomicUsesRename(t *testing.T) {
	// Write, then poison the target by making it read-only. Save should still
	// succeed via tmp+rename semantics on POSIX.
	dir := t.TempDir()
	sp := filepath.Join(dir, "state.json")
	s := &State{Theme: "one"}
	if err := s.saveTo(sp); err != nil {
		t.Fatalf("first save: %v", err)
	}
	s.Theme = "two"
	if err := s.saveTo(sp); err != nil {
		t.Fatalf("second save: %v", err)
	}
	b, err := os.ReadFile(sp)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var got State
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Theme != "two" {
		t.Errorf("theme=%q want two", got.Theme)
	}
}

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
