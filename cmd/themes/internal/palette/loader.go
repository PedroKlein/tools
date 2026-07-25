package palette

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Load parses a theme.json file and returns a fully-hydrated *Theme
// with all optional fields defaulted per the schema documentation.
//
// The `path` argument is either the theme directory (looked up
// <path>/theme.json) or the theme.json file itself.
//
// After Load returns nil error:
//   - t.Palette.Ansi is length 16, every entry non-empty
//   - t.Palette.Semantic.* (7 required slots) are non-empty
//   - All optional slots carry their documented default
//   - t.Dir is absolute
//   - t.Unknown contains every top-level key the schema did not define
//     (empty map when there are none)
func Load(path string) (*Theme, error) {
	filePath, dir, err := resolveThemePaths(path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return decodeTheme(f, dir)
}

// resolveThemePaths accepts either a directory containing theme.json or
// a direct path to a theme.json file. Returns (fileAbs, dirAbs, error).
func resolveThemePaths(path string) (file, dir string, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return filepath.Join(abs, "theme.json"), abs, nil
	}
	// Direct file
	return abs, filepath.Dir(abs), nil
}

// decodeTheme reads theme.json from r, unmarshals into *Theme, captures
// unknown top-level keys into t.Unknown, then default-fills every
// optional slot. Exposed for testing (avoids touching the filesystem).
func decodeTheme(r io.Reader, dir string) (*Theme, error) {
	// Two-pass parse:
	//   1. Into map[string]any for unknown-key preservation.
	//   2. Marshal that back and Decode into *Theme via json (struct
	//      tags do the field mapping).
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("theme.json is not valid JSON: %w", err)
	}

	// Snapshot known top-level keys so we can populate t.Unknown with
	// everything else. Kept in sync with Theme's json tags.
	known := map[string]struct{}{
		"$schema":     {},
		"name":        {},
		"displayName": {},
		"author":      {},
		"upstream":    {},
		"appearance":  {},
		"description": {},
		"palette":     {},
		"typography":  {},
		"effects":     {},
		"macos":       {},
		"wallpapers":  {},
		"hints":       {},
		"overrides":   {},
	}
	unknown := map[string]any{}
	for k, v := range doc {
		if _, ok := known[k]; !ok {
			unknown[k] = v
		}
	}

	var t Theme
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	if err := dec.Decode(&t); err != nil {
		return nil, fmt.Errorf("decode theme.json: %w", err)
	}
	t.Dir = dir
	t.Unknown = unknown

	if err := validateRequired(&t); err != nil {
		return nil, err
	}
	fillDefaults(&t)
	return &t, nil
}

// validateRequired enforces the required-field invariants the schema
// checks. Duplicating them here means callers who bypass the schema
// (e.g. tests) still get a coherent error.
func validateRequired(t *Theme) error {
	if t.Name == "" {
		return errors.New("theme.json: name is required")
	}
	if t.Appearance != "dark" && t.Appearance != "light" {
		return fmt.Errorf("theme.json: appearance must be dark or light, got %q", t.Appearance)
	}
	for i, c := range t.Palette.Ansi {
		if c == "" {
			return fmt.Errorf("theme.json: palette.ansi[%d] is empty (need 16 hex colors)", i)
		}
	}
	s := t.Palette.Semantic
	for name, val := range map[string]string{
		"bg":      s.Bg,
		"fg":      s.Fg,
		"muted":   s.Muted,
		"accent":  s.Accent,
		"error":   s.Error,
		"warning": s.Warning,
		"ok":      s.Ok,
	} {
		if val == "" {
			return fmt.Errorf("theme.json: palette.semantic.%s is required", name)
		}
	}
	return nil
}

// fillDefaults populates every optional field with its documented default.
// After this runs, every Theme field has a usable value.
func fillDefaults(t *Theme) {
	if t.DisplayName == "" {
		t.DisplayName = t.Name
	}
	fillSemanticDefaults(&t.Palette.Semantic, t.Palette.Ansi)
	fillGradientsDefaults(&t.Palette.Gradients, t.Palette.Semantic)
	fillEffectsDefaults(&t.Effects)
	fillMacosDefaults(&t.Macos, t.Appearance, t.Palette.Semantic)
	fillWallpapersDefaults(&t.Wallpapers)
	if t.Hints == nil {
		t.Hints = map[string]any{}
	}
	if t.Overrides == nil {
		t.Overrides = map[string]string{}
	}
	if t.Unknown == nil {
		t.Unknown = map[string]any{}
	}
}

func fillSemanticDefaults(s *Semantic, ansi [16]string) {
	// Simple defaulted slots.
	if s.Accent2 == "" {
		s.Accent2 = s.Accent
	}
	if s.Info == "" {
		s.Info = s.Accent
	}
	if s.SelectionBg == "" {
		s.SelectionBg = s.Accent
	}
	if s.SelectionFg == "" {
		s.SelectionFg = s.Fg
	}
	if s.Cursor == "" {
		s.Cursor = s.Accent2
	}
	if s.Border == "" {
		s.Border = s.Muted
	}
	if s.BgAlt == "" {
		s.BgAlt = darken(s.Bg, 0.05)
	}
	if s.FgDim == "" {
		s.FgDim = mix(s.Fg, s.Muted, 0.5)
	}

	// Git colors — derive from ok/error/warning.
	if s.Git.Added == "" {
		s.Git.Added = s.Ok
	}
	if s.Git.Removed == "" {
		s.Git.Removed = s.Error
	}
	if s.Git.Modified == "" {
		s.Git.Modified = s.Warning
	}

	// Syntax colors — derive from the ANSI palette when unset.
	// ANSI ordering: 0=black, 1=red, 2=green, 3=yellow, 4=blue, 5=magenta,
	// 6=cyan, 7=white; 8-15 = bright variants.
	if s.Syntax.Keyword == "" {
		s.Syntax.Keyword = ansi[13] // brMagenta
	}
	if s.Syntax.String == "" {
		s.Syntax.String = ansi[10] // brGreen
	}
	if s.Syntax.Number == "" {
		s.Syntax.Number = ansi[6] // cyan
	}
	if s.Syntax.Comment == "" {
		s.Syntax.Comment = s.Muted
	}
	if s.Syntax.Type == "" {
		s.Syntax.Type = s.Accent
	}
	if s.Syntax.Function == "" {
		s.Syntax.Function = ansi[3] // yellow
	}
	if s.Syntax.Operator == "" {
		s.Syntax.Operator = s.Fg
	}
}

func fillGradientsDefaults(g *Gradients, s Semantic) {
	filled := func(t [3]string) bool { return t[0] != "" && t[1] != "" && t[2] != "" }
	if !filled(g.Temp) {
		g.Temp = [3]string{s.Warning, s.Accent, s.Ok}
	}
	if !filled(g.Cpu) {
		g.Cpu = [3]string{s.Ok, s.Accent, s.Warning}
	}
	if !filled(g.Memory) {
		g.Memory = [3]string{s.Ok, s.Warning, s.Error}
	}
	if !filled(g.Network) {
		g.Network = [3]string{s.Accent, s.Info, s.Warning}
	}
}

func fillEffectsDefaults(e *Effects) {
	// Sentinel: opacity 0 == unset (0 is meaningless for a terminal; a
	// literal 0 would produce an invisible background).
	if e.Opacity == 0 {
		e.Opacity = 1.0
	}
	if e.Cursor.Shape == "" {
		e.Cursor.Shape = "block"
	}
}

func fillMacosDefaults(m *Macos, appearance string, s Semantic) {
	if m.Appearance == "" {
		m.Appearance = appearance
	}
	if m.Highlight == "" {
		m.Highlight = s.Accent
	}
	// Accent left empty: the .macos.json emitter (or hue-matcher in a
	// later phase) resolves via macos_preset.go against s.Accent.
}

func fillWallpapersDefaults(w *Wallpapers) {
	if w.Placement == "" {
		w.Placement = "fill"
	}
	if w.Default == "" && len(w.List) > 0 {
		w.Default = w.List[0].File
	}
}
