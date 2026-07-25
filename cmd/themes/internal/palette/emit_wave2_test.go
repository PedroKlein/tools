package palette

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmittersWave2SmokeAgainstOsakaJade runs the 4 wave-2 emitters
// against osaka-jade and asserts each output has correct semantic
// content plus (for pi.json) valid JSON.
func TestEmittersWave2SmokeAgainstOsakaJade(t *testing.T) {
	th, err := LoadTheme(filepath.Join("testdata", "osaka-jade"))
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	got := map[string]Emitter{}
	for _, e := range EmittersV4 {
		got[e.App()] = e
	}

	t.Run("nvim", func(t *testing.T) {
		e := got["nvim"]
		if e == nil {
			t.Fatal("nvim emitter missing")
		}
		var buf bytes.Buffer
		if err := e.Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		out := buf.String()
		// Baseline + semantic markers with -- (Lua) prefix.
		if !strings.Contains(out, "-- --- semantic ---") {
			t.Errorf("missing semantic marker with -- prefix:\n%s", out)
		}
		// Correct hint: colorscheme is bamboo (from osaka-jade hints).
		if !strings.Contains(out, `vim.cmd.colorscheme, "bamboo"`) {
			t.Errorf("expected bamboo colorscheme call:\n%s", out)
		}
		if !strings.Contains(out, `vim.cmd.colorscheme, "carbonfox"`) {
			t.Errorf("expected carbonfox fallback:\n%s", out)
		}
		// Sidecar contents should be appended.
		if !strings.Contains(out, "bamboo.setup") {
			t.Errorf("nvim override sidecar not appended:\n%s", out)
		}
	})

	t.Run("pi", func(t *testing.T) {
		e := got["pi"]
		if e == nil {
			t.Fatal("pi emitter missing")
		}
		var buf bytes.Buffer
		if err := e.Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		// Must be valid JSON (pi emitter skips block markers).
		var m map[string]any
		if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
			t.Fatalf("pi.json is not valid JSON: %v\n%s", err, buf.String())
		}
		if m["name"] != "osaka-jade" {
			t.Errorf("pi.name = %v, want osaka-jade", m["name"])
		}
		vars := m["vars"].(map[string]any)
		if vars["bg"] != th.Palette.Semantic.Bg {
			t.Errorf("pi.vars.bg = %v, want %s", vars["bg"], th.Palette.Semantic.Bg)
		}
	})

	t.Run("fzf", func(t *testing.T) {
		var buf bytes.Buffer
		if err := got["fzf"].Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		if !strings.Contains(buf.String(), "FZF_DEFAULT_OPTS_COLORS=") {
			t.Errorf("missing FZF_DEFAULT_OPTS_COLORS:\n%s", buf.String())
		}
		if !strings.Contains(buf.String(), th.Palette.Semantic.Accent) {
			t.Errorf("fzf output missing accent color %s", th.Palette.Semantic.Accent)
		}
	})

	t.Run("zsh-syntax-highlight", func(t *testing.T) {
		var buf bytes.Buffer
		if err := got["zsh-syntax-highlight"].Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		out := buf.String()
		for _, scope := range []string{"comment", "reserved-word", "path", "bracket-level-1"} {
			if !strings.Contains(out, "ZSH_HIGHLIGHT_STYLES["+scope+"]") {
				t.Errorf("missing zsh scope %q:\n%s", scope, out)
			}
		}
	})
}
