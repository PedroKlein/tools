package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteMetaWritesToThemeJSON verifies AC-1 for P5.4: settings pane
// writes back to theme.json (not palette.toml).
func TestWriteMetaWritesToThemeJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.json")

	// Minimal valid theme.
	initial := `{
  "name": "test",
  "appearance": "dark",
  "palette": {
    "ansi": ["#000","#800000","#008000","#808000","#000080","#800080","#008080","#c0c0c0","#808080","#f00","#0f0","#ff0","#00f","#f0f","#0ff","#fff"],
    "semantic": {"bg":"#111","fg":"#eee","muted":"#888","accent":"#7a8","error":"#f55","warning":"#ec7","ok":"#5a8"}
  }
}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeMeta(dir, map[string]string{"opacity": "0.9", "blur": "15", "mode": "dark"}); err != nil {
		t.Fatalf("writeMeta: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read theme.json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("theme.json is not valid JSON after writeMeta: %v", err)
	}
	effects := doc["effects"].(map[string]any)
	if effects["opacity"].(float64) != 0.9 {
		t.Errorf("effects.opacity = %v, want 0.9", effects["opacity"])
	}
	if effects["blur"].(float64) != 15 {
		t.Errorf("effects.blur = %v, want 15", effects["blur"])
	}
	macos := doc["macos"].(map[string]any)
	if macos["appearance"] != "dark" {
		t.Errorf("macos.appearance = %v, want dark", macos["appearance"])
	}
}

// TestWriteMetaPreservesUnknownTopLevelKeys verifies AC-2 for P5.4:
// unknown top-level keys survive the settings-pane write-back cycle.
func TestWriteMetaPreservesUnknownTopLevelKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.json")

	initial := `{
  "name": "test",
  "appearance": "dark",
  "experimental_key": "future-proof",
  "another_field": {"nested": true, "count": 42},
  "palette": {
    "ansi": ["#000","#800","#080","#880","#008","#808","#088","#ccc","#888","#f00","#0f0","#ff0","#00f","#f0f","#0ff","#fff"],
    "semantic": {"bg":"#111","fg":"#eee","muted":"#888","accent":"#7a8","error":"#f55","warning":"#ec7","ok":"#5a8"}
  }
}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeMeta(dir, map[string]string{"opacity": "0.85"}); err != nil {
		t.Fatalf("writeMeta: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON after write: %v", err)
	}
	if doc["experimental_key"] != "future-proof" {
		t.Errorf("experimental_key lost: %v", doc["experimental_key"])
	}
	af, ok := doc["another_field"].(map[string]any)
	if !ok {
		t.Fatalf("another_field lost: %+v", doc)
	}
	if af["nested"] != true || af["count"].(float64) != 42 {
		t.Errorf("another_field.{nested,count} lost: %+v", af)
	}
	// Sanity: palette also preserved.
	if _, ok := doc["palette"].(map[string]any); !ok {
		t.Errorf("palette lost")
	}
}

// TestLoadMetaReadsTheme documents that loadMeta pulls from theme.json.
func TestLoadMetaReadsTheme(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.json")

	// Populated theme.json.
	initial := `{
  "name": "test",
  "appearance": "dark",
  "palette": {
    "ansi": ["#000","#800","#080","#880","#008","#808","#088","#ccc","#888","#f00","#0f0","#ff0","#00f","#f0f","#0ff","#fff"],
    "semantic": {"bg":"#111","fg":"#eee","muted":"#888","accent":"#7a8","error":"#f55","warning":"#ec7","ok":"#5a8"}
  },
  "effects": {"opacity": 0.85, "blur": 20},
  "macos": {"appearance": "light", "accent": "green"}
}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	meta := loadMeta(dir)
	if !strings.HasPrefix(meta["opacity"], "0.85") {
		t.Errorf("opacity = %q, want 0.85", meta["opacity"])
	}
	if meta["blur"] != "20" {
		t.Errorf("blur = %q, want 20", meta["blur"])
	}
	if meta["mode"] != "light" {
		t.Errorf("mode = %q, want light (from macos.appearance override)", meta["mode"])
	}
	if meta["accent_preset"] != "green" {
		t.Errorf("accent_preset = %q, want green", meta["accent_preset"])
	}
}
