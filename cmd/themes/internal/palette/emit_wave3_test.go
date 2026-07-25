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
	th, err := Load(filepath.Join("testdata", "osaka-jade"))
	if err != nil {
		t.Fatalf("Load: %v", err)
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
			// Regression: line backgrounds must be subtle mix(bg,diff-hue)
			// tints, not full-strength Git.Added/Git.Removed. Full-strength
			// paints the whole diff line in a bright red/green wash — too
			// loud, obscures the code.
			"minus-style = syntax",
			"plus-style = syntax",
			"minus-emph-style = syntax",
			"plus-emph-style = syntax",
			`file-style = "` + th.Palette.Semantic.Accent + `"`,
			// Regression: `#hex` starts a comment in git-config INI syntax
			// so the palette must be a quoted single string, not raw
			// space-separated tokens. Delta parses the quoted value by
			// re-splitting on whitespace.
			`blame-palette = "`,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q:\n%s", want, out)
			}
		}
		// Full-strength Git.Added/Removed MUST NOT appear as the bg value
		// on minus/plus-style. They still appear on line-numbers-*-style
		// (single-glyph pop against the tinted bg), so we specifically
		// check the line-background style lines.
		if strings.Contains(out, "minus-style = syntax \""+th.Palette.Semantic.Git.Removed+"\"") ||
			strings.Contains(out, "plus-style = syntax \""+th.Palette.Semantic.Git.Added+"\"") {
			t.Errorf("delta minus/plus-style uses full-strength diff hue as bg (too loud); expected mix() tint")
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
		// Regression (e1): the baseline emits a top-level `gui:` mapping
		// and the semantic block emits its `theme:` child. Two top-level
		// `gui:` keys are invalid YAML — lazygit rejects with
		// 'mapping key "gui" already defined'. Guard by asserting a
		// SINGLE ^gui: line in the output.
		guiKeys := 0
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "gui:") {
				guiKeys++
			}
		}
		if guiKeys != 1 {
			t.Errorf("lazygit.yml has %d top-level `gui:` keys, want 1:\n%s", guiKeys, out)
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
