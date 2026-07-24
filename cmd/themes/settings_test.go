package main
import (
  "os"
  "path/filepath"
  "testing"
)

func TestWriteMetaPreservesUnrelatedKeys(t *testing.T) {
	// Regression: earlier writeMeta replaced the whole [meta] block with
	// only the update keys, dropping any pre-existing entries (e.g. a user's
	// hand-authored `accent_preset = "Blue"` override would be lost when
	// the TUI adjusted opacity). Fix: merge instead of replace.
	dir := t.TempDir()
	path := filepath.Join(dir, "palette.toml")
	if err := os.WriteFile(path, []byte(`[vars]
accent = "#549E6A"

[meta]
mode = "dark"
accent_preset = "Blue"
custom_knob = "keep-me"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeMeta(dir, map[string]string{"opacity": "0.9"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	if !containsSubstr(s, `opacity = 0.9`) {
		t.Errorf("opacity not added: %q", s)
	}
	if !containsSubstr(s, `mode = "dark"`) {
		t.Errorf("pre-existing mode lost: %q", s)
	}
	if !containsSubstr(s, `accent_preset = "Blue"`) {
		t.Errorf("pre-existing accent_preset lost: %q", s)
	}
	if !containsSubstr(s, `custom_knob = "keep-me"`) {
		t.Errorf("pre-existing custom_knob lost: %q", s)
	}
}

func TestWriteMetaCreatesAndUpdates(t *testing.T) {
  dir := t.TempDir()
  path := filepath.Join(dir, "palette.toml")
  // Start with a real-ish palette.toml
  os.WriteFile(path, []byte(`[vars]
accent = "#549E6A"

[roles]
accent = "accent"
`), 0644)
  if err := writeMeta(dir, map[string]string{"opacity": "0.9", "blur": "15", "mode": "dark"}); err != nil {
    t.Fatal(err)
  }
  data, _ := os.ReadFile(path)
  s := string(data)
  if !containsSubstr(s, "opacity = 0.9") || !containsSubstr(s, "blur = 15") || !containsSubstr(s, `mode = "dark"`) {
    t.Errorf("meta not written:\n%s", s)
  }
  if !containsSubstr(s, "[vars]") || !containsSubstr(s, "[roles]") {
    t.Errorf("existing sections lost:\n%s", s)
  }
}

func containsSubstr(a, b string) bool {
  for i := 0; i+len(b) <= len(a); i++ {
    if a[i:i+len(b)] == b {
      return true
    }
  }
  return false
}
