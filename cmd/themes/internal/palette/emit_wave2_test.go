package palette

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmittersWave2SmokeAgainstOsakaJade runs the wave-2 emitters
// against osaka-jade and asserts each output has correct semantic
// content plus valid agent theme JSON.
func TestEmittersWave2SmokeAgainstOsakaJade(t *testing.T) {
	th, err := Load(filepath.Join("testdata", "osaka-jade"))
	if err != nil {
		t.Fatalf("Load: %v", err)
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
		if e.Filename() != "neovim.lua" {
			t.Errorf("Filename() = %q, want neovim.lua (LazyVim's colorscheme.lua reader expects this exact name)", e.Filename())
		}
		var buf bytes.Buffer
		if err := e.Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		out := buf.String()
		// Regression (e2): file must be a Lua plugin spec table,
		// not an imperative script. LazyVim dofile()'s the file and
		// expects a table return value.
		if !strings.Contains(out, "return {") {
			t.Errorf("nvim emitter must return a plugin spec table:\n%s", out)
		}
		if !strings.Contains(out, "config = function()") {
			t.Errorf("nvim spec must have config = function() block:\n%s", out)
		}
		// Plugin repo from hints.nvim.plugin appears as spec[1].
		if !strings.Contains(out, `"ribru17/bamboo.nvim"`) {
			t.Errorf("expected hints.nvim.plugin (ribru17/bamboo.nvim) as spec[1]:\n%s", out)
		}
		// Sidecar body injected into config function — the osaka-jade
		// override calls bamboo.setup with custom colors.
		if !strings.Contains(out, "bamboo.setup") {
			t.Errorf("expected sidecar bamboo.setup body inside config function:\n%s", out)
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

		// Regression (d1-pi-emitter): Pi's runtime rejects themes missing
		// any of these keys with "missing required color tokens". The v4
		// emitter was a strict subset of v3 and dropped 14 keys, silently
		// breaking every Pi session after v4 migration.
		colors := m["colors"].(map[string]any)
		required := []string{
			// v3 keys still needed
			"accent", "border", "borderAccent", "borderMuted",
			"success", "error", "warning", "muted", "dim",
			"selectedBg", "userMessageBg",
			"toolPendingBg", "toolSuccessBg", "toolErrorBg", "toolOutput",
			"mdHeading", "mdLink", "mdLinkUrl", "mdCode",
			"mdCodeBlock", "mdCodeBlockBorder", "mdQuote", "mdQuoteBorder",
			"mdHr", "mdListBullet",
			"toolDiffAdded", "toolDiffRemoved", "toolDiffContext",
			"syntaxComment", "syntaxKeyword", "syntaxFunction",
			"syntaxString", "syntaxNumber", "syntaxType", "syntaxOperator",
			"bashMode",
			// d1 additions — must be present or Pi rejects the theme
			"text", "userMessageText",
			"customMessageBg", "customMessageLabel", "customMessageText",
			"toolTitle",
			"syntaxPunctuation", "syntaxVariable",
			"thinkingOff", "thinkingMinimal", "thinkingLow",
			"thinkingMedium", "thinkingHigh", "thinkingXhigh",
		}
		var missing []string
		for _, k := range required {
			if _, ok := colors[k]; !ok {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			t.Errorf("pi.colors missing %d required keys: %v", len(missing), missing)
		}

		// Regression (F1): v3 kept these 4 slots as empty strings so
		// Pi's runtime picks its own semantic defaults (usually 'fg'
		// but sometimes brighter for titles). Emitting 'fg' overrides
		// that logic with a flat foreground color, making titles blend
		// with body text. Match v3: 4 specific slots must be "".
		for _, k := range []string{"text", "userMessageText", "customMessageText", "toolTitle"} {
			if v, ok := colors[k].(string); !ok || v != "" {
				t.Errorf("pi.colors.%s = %q, want \"\" (Pi runtime picks a context-aware default)", k, v)
			}
		}
	})

	t.Run("omp", func(t *testing.T) {
		e := got["omp"]
		if e == nil {
			t.Fatal("omp emitter missing")
		}
		if e.Filename() != "omp.json" {
			t.Errorf("Filename() = %q, want omp.json", e.Filename())
		}
		var buf bytes.Buffer
		if err := e.Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}

		var m map[string]any
		if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
			t.Fatalf("omp.json is not valid JSON: %v\n%s", err, buf.String())
		}
		if m["name"] != "osaka-jade" {
			t.Errorf("omp.name = %v, want osaka-jade", m["name"])
		}
		vars := m["vars"].(map[string]any)
		if vars["bg"] != th.Palette.Semantic.Bg {
			t.Errorf("omp.vars.bg = %v, want %s", vars["bg"], th.Palette.Semantic.Bg)
		}
		colors := m["colors"].(map[string]any)
		for _, k := range []string{"accent", "text", "thinkingText", "selectedBg", "statusLineModel", "statusLineSubagents"} {
			if _, ok := colors[k]; !ok {
				t.Errorf("omp.colors missing %s", k)
			}
		}
		symbols := m["symbols"].(map[string]any)
		if symbols["preset"] != "nerd" {
			t.Errorf("omp.symbols.preset = %v, want nerd", symbols["preset"])
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
