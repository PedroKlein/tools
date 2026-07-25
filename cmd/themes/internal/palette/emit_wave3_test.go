package palette

import (
	"bytes"
	"encoding/xml"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmittersWave3SmokeAgainstOsakaJade runs delta/bat/lazygit/gh-dash
// against osaka-jade and asserts each output has correct semantic
// content. bat.tmTheme is also verified as valid XML.
func TestEmittersWave3SmokeAgainstOsakaJade(t *testing.T) {
	th, err := LoadTheme(filepath.Join("testdata", "osaka-jade"))
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	got := map[string]Emitter{}
	for _, e := range EmittersV4 {
		got[e.App()] = e
	}

	t.Run("delta", func(t *testing.T) {
		var buf bytes.Buffer
		if err := got["delta"].Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		out := buf.String()
		for _, want := range []string{
			"[delta]",
			"plus-style = normal",
			"minus-style = normal",
			`file-style = "` + th.Palette.Semantic.Accent + `"`,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("bat", func(t *testing.T) {
		var buf bytes.Buffer
		if err := got["bat"].Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		// AC-2: bat XML must be parseable as XML.
		var v any
		if err := xml.Unmarshal(buf.Bytes(), &v); err != nil {
			t.Fatalf("bat.tmTheme is not valid XML: %v\n%s", err, buf.String())
		}
		if !strings.Contains(buf.String(), th.Palette.Semantic.Bg) {
			t.Errorf("bat.tmTheme missing bg color")
		}
	})

	t.Run("lazygit", func(t *testing.T) {
		var buf bytes.Buffer
		if err := got["lazygit"].Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		out := buf.String()
		for _, want := range []string{"gui:", "theme:", "activeBorderColor:", "selectedLineBgColor:"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("gh-dash", func(t *testing.T) {
		var buf bytes.Buffer
		if err := got["gh-dash"].Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		out := buf.String()
		for _, want := range []string{"theme:", "ui:", "colors:", "background:", "border:"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q:\n%s", want, out)
			}
		}
	})
}
