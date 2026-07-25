package palette

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadReturnsHydratedTheme verifies AC-1: Load returns a
// fully-hydrated Theme with non-zero values for required fields, and
// applies documented defaults for missing optional fields.
func TestLoadReturnsHydratedTheme(t *testing.T) {
	dir := filepath.Join("testdata", "osaka-jade")
	th, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Required fields present and populated.
	if th.Name != "osaka-jade" {
		t.Errorf("Name = %q, want osaka-jade", th.Name)
	}
	if th.Appearance != "dark" {
		t.Errorf("Appearance = %q, want dark", th.Appearance)
	}
	if th.Dir == "" || !filepath.IsAbs(th.Dir) {
		t.Errorf("Dir must be absolute, got %q", th.Dir)
	}

	// All 16 ANSI entries hydrated.
	for i, c := range th.Palette.Ansi {
		if c == "" {
			t.Errorf("Ansi[%d] is empty", i)
		}
	}

	// All 7 required semantic slots hydrated.
	s := th.Palette.Semantic
	for name, val := range map[string]string{
		"bg": s.Bg, "fg": s.Fg, "muted": s.Muted, "accent": s.Accent,
		"error": s.Error, "warning": s.Warning, "ok": s.Ok,
	} {
		if val == "" {
			t.Errorf("Semantic.%s is empty", name)
		}
	}

	// osaka-jade defines every optional field; make sure they survived
	// the round-trip.
	if th.Palette.Semantic.Cursor != "#D7C995" {
		t.Errorf("Cursor = %q, want #D7C995", th.Palette.Semantic.Cursor)
	}
	if th.Palette.Gradients.Temp[0] == "" {
		t.Error("Gradients.Temp not hydrated")
	}
	if th.Effects.Opacity != 0.85 {
		t.Errorf("Effects.Opacity = %v, want 0.85", th.Effects.Opacity)
	}
	if th.Macos.Accent != "green" {
		t.Errorf("Macos.Accent = %q, want green", th.Macos.Accent)
	}
	if th.Wallpapers.Default != "osaka-jade-bg.jpg" {
		t.Errorf("Wallpapers.Default = %q", th.Wallpapers.Default)
	}
}

// TestLoadDefaultsFillEmptySlots verifies AC-1: default-fill logic
// populates optional slots when they are absent from theme.json.
func TestLoadDefaultsFillEmptySlots(t *testing.T) {
	// Minimal theme: 7 required semantic slots and 16 ANSI, nothing else.
	minimal := `{
		"name": "minimal",
		"appearance": "dark",
		"palette": {
			"ansi": [
				"#000000","#800000","#008000","#808000",
				"#000080","#800080","#008080","#c0c0c0",
				"#808080","#ff0000","#00ff00","#ffff00",
				"#0000ff","#ff00ff","#00ffff","#ffffff"
			],
			"semantic": {
				"bg":"#111","fg":"#eee","muted":"#888",
				"accent":"#7a8","error":"#f55","warning":"#ec7","ok":"#5a8"
			}
		}
	}`
	th, err := decodeTheme(strings.NewReader(minimal), "/tmp/minimal")
	if err != nil {
		t.Fatalf("decodeTheme: %v", err)
	}

	// DisplayName defaults to Name.
	if th.DisplayName != "minimal" {
		t.Errorf("DisplayName = %q, want fallback to Name", th.DisplayName)
	}

	// Optional semantic slots default from required ones.
	s := th.Palette.Semantic
	if s.Accent2 != s.Accent {
		t.Errorf("Accent2 = %q, want default to Accent %q", s.Accent2, s.Accent)
	}
	if s.Info != s.Accent {
		t.Errorf("Info = %q, want default to Accent %q", s.Info, s.Accent)
	}
	if s.Cursor != s.Accent2 {
		t.Errorf("Cursor = %q, want default to Accent2 %q", s.Cursor, s.Accent2)
	}
	if s.Border != s.Muted {
		t.Errorf("Border = %q, want default to Muted %q", s.Border, s.Muted)
	}
	if s.BgAlt == "" || s.BgAlt == s.Bg {
		t.Errorf("BgAlt should be derived (darken bg), got %q vs bg %q", s.BgAlt, s.Bg)
	}

	// Git colors default from ok/error/warning.
	if s.Git.Added != s.Ok || s.Git.Removed != s.Error || s.Git.Modified != s.Warning {
		t.Errorf("Git defaults not applied: %+v", s.Git)
	}

	// Syntax colors non-empty.
	if s.Syntax.Comment == "" || s.Syntax.String == "" {
		t.Errorf("Syntax defaults not applied: %+v", s.Syntax)
	}

	// Gradients derived from semantic slots (all 3 non-empty).
	if th.Palette.Gradients.Temp[0] == "" || th.Palette.Gradients.Cpu[0] == "" {
		t.Errorf("Gradients defaults not applied: %+v", th.Palette.Gradients)
	}

	// Effects.Opacity defaults to 1.0 (fully opaque, sentinel-based).
	if th.Effects.Opacity != 1.0 {
		t.Errorf("Effects.Opacity default = %v, want 1.0", th.Effects.Opacity)
	}
	if th.Effects.Cursor.Shape != "block" {
		t.Errorf("Cursor.Shape default = %q, want block", th.Effects.Cursor.Shape)
	}

	// macOS defaults.
	if th.Macos.Appearance != "dark" {
		t.Errorf("Macos.Appearance = %q, want mirror top-level dark", th.Macos.Appearance)
	}
	if th.Macos.Highlight != s.Accent {
		t.Errorf("Macos.Highlight = %q, want fallback to Accent", th.Macos.Highlight)
	}

	// Wallpapers.Placement defaults to fill.
	if th.Wallpapers.Placement != "fill" {
		t.Errorf("Wallpapers.Placement = %q, want fill", th.Wallpapers.Placement)
	}

	// Empty maps rather than nil.
	if th.Hints == nil || th.Overrides == nil || th.Unknown == nil {
		t.Error("Hints/Overrides/Unknown should default to empty maps, not nil")
	}
}

// TestLoadPreservesUnknownKeys verifies AC-2: unknown top-level keys
// are preserved (not errored on) so schema evolution doesn't break older
// tooling reading newer themes.
func TestLoadPreservesUnknownKeys(t *testing.T) {
	// Same minimal skeleton with an extra top-level key.
	raw := `{
		"name": "with-extra",
		"appearance": "dark",
		"experimental_key": "future-value",
		"another_new_field": {"nested": true},
		"palette": {
			"ansi": [
				"#000000","#800000","#008000","#808000",
				"#000080","#800080","#008080","#c0c0c0",
				"#808080","#ff0000","#00ff00","#ffff00",
				"#0000ff","#ff00ff","#00ffff","#ffffff"
			],
			"semantic": {
				"bg":"#111","fg":"#eee","muted":"#888",
				"accent":"#7a8","error":"#f55","warning":"#ec7","ok":"#5a8"
			}
		}
	}`
	th, err := decodeTheme(strings.NewReader(raw), "/tmp/with-extra")
	if err != nil {
		t.Fatalf("decodeTheme rejected unknown key: %v", err)
	}
	if got, ok := th.Unknown["experimental_key"]; !ok || got != "future-value" {
		t.Errorf("Unknown[experimental_key] = %v, want future-value", got)
	}
	if _, ok := th.Unknown["another_new_field"]; !ok {
		t.Errorf("Unknown[another_new_field] missing: %v", th.Unknown)
	}
	// Known keys must NOT be in Unknown.
	for _, k := range []string{"name", "palette", "appearance"} {
		if _, ok := th.Unknown[k]; ok {
			t.Errorf("Unknown incorrectly contains known key %q", k)
		}
	}
}

// TestLoadRejectsMissingRequired documents that the loader enforces
// required fields even when callers bypass the schema.
func TestLoadRejectsMissingRequired(t *testing.T) {
	// Missing palette.semantic.bg
	raw := `{
		"name": "bad",
		"appearance": "dark",
		"palette": {
			"ansi": [
				"#000000","#800000","#008000","#808000",
				"#000080","#800080","#008080","#c0c0c0",
				"#808080","#ff0000","#00ff00","#ffff00",
				"#0000ff","#ff00ff","#00ffff","#ffffff"
			],
			"semantic": {
				"fg":"#eee","muted":"#888",
				"accent":"#7a8","error":"#f55","warning":"#ec7","ok":"#5a8"
			}
		}
	}`
	_, err := decodeTheme(strings.NewReader(raw), "/tmp/bad")
	if err == nil {
		t.Fatal("expected error for missing semantic.bg")
	}
	if !strings.Contains(err.Error(), "bg") {
		t.Errorf("error should mention missing field bg: %v", err)
	}
}

// TestOverridePathResolvesRelative verifies the sidecar-path helper.
func TestOverridePathResolvesRelative(t *testing.T) {
	th := &Theme{
		Dir:       "/tmp/theme-x",
		Overrides: map[string]string{"nvim_path": "overrides/nvim.lua"},
	}
	got := th.OverridePath("nvim")
	want := "/tmp/theme-x/overrides/nvim.lua"
	if got != want {
		t.Errorf("OverridePath = %q, want %q", got, want)
	}

	// Absolute path is preserved.
	th.Overrides["ghostty_path"] = "/etc/ghostty.local"
	if got := th.OverridePath("ghostty"); got != "/etc/ghostty.local" {
		t.Errorf("absolute OverridePath = %q, want /etc/ghostty.local", got)
	}

	// Missing key returns empty.
	if got := th.OverridePath("no-such-app"); got != "" {
		t.Errorf("missing override should return \"\", got %q", got)
	}
}
