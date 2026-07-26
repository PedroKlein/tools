package palette

import (
	"encoding/json"
	"fmt"
	"io"
)

// v4 pi emitter — reads theme.json, writes pi.json.
//
// pi.json is consumed by the pi coding agent. The schema is fixed by
// the pi-mono repo (~/.pi/agent/npm/node_modules/pi/theme-schema.json).
// v4 output is a superset of v3: same var and color keys, populated
// from theme.json's semantic slots.
//
// TRANSPARENCY POLICY (docs/plans/theme-transparency.md § P4.pi)
// ==============================================================
// All `*Bg` color fields default to "" (empty string) so pi's TUI
// never paints an explicit background color for chat messages, tool
// blocks, or selected items. Empty is the schema convention for
// "transparent/default" (same as `text`, `userMessageText`, etc.).
//
// This is what enables Ghostty's `background-opacity-cells = true`
// policy to render pi's cells uniformly translucent — identical to
// surrounding empty pane cells. Painting bg=#mantle etc. produced a
// visible "pi region" with slightly different opacity blending than
// the surrounding terminal.
//
// Visual differentiation the *Bg fields used to provide (message-type
// coloring, tool status pills) is now lost. Themes that want it back
// can set hints.pi.colors.<field> to a palette var; the hint override
// path below picks it up. Label fg colors (customMessageLabel, etc.)
// still provide color signal for at-a-glance scanning.

type piEmitter struct{}

func (piEmitter) App() string      { return "pi" }
func (piEmitter) Filename() string { return "pi.json" }

// Emit produces JSON, which does not tolerate # comments. We bypass
// EmitStandard's block markers and emit the JSON directly.
func (piEmitter) Emit(t *Theme, w io.Writer) error {
	return emitPiJSON(t, w)
}

// piThemeV4 is the wire shape of pi.json.
type piThemeV4 struct {
	Schema string            `json:"$schema"`
	Name   string            `json:"name"`
	Vars   map[string]string `json:"vars"`
	Colors map[string]string `json:"colors"`
	Export map[string]string `json:"export,omitempty"`
}

func emitPiJSON(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	a := t.Palette.Ansi

	vars := map[string]string{
		"bg":       s.Bg,
		"mantle":   s.BgAlt,
		"surface0": s.SelectionBg,
		"surface1": s.Border,
		"surface2": s.FgDim,
		"accent":   s.Accent,
		"bright":   s.Accent2,
		"muted":    s.Muted,
		"fg":       s.Fg,
		"white":    a[15],
		"red":      s.Error,
		"yellow":   s.Warning,
		"green":    s.Ok,
		"cyan":     a[6],
		"blue":     s.Info,
		"purple":   s.Syntax.Keyword,
	}

	colors := map[string]string{
		"accent":            "accent",
		"border":            "surface1",
		"borderAccent":      "bright",
		"borderMuted":       "surface1",
		"success":           "bright",
		"error":             "red",
		"warning":           "yellow",
		"muted":             "muted",
		"dim":               "muted",
		"text":              "",
		"userMessageBg":     "", // transparent (see file header)
		"userMessageText":   "",
		"customMessageBg":   "", // transparent
		"customMessageLabel": "purple",
		"customMessageText": "",
		"selectedBg":        "", // transparent — selection uses reverse/bold at pi's rendering layer
		"toolPendingBg":     "", // transparent; toolTitle fg gives status signal
		"toolSuccessBg":     "",
		"toolErrorBg":       "",
		"toolOutput":        "muted",
		"toolTitle":         "",
		"mdHeading":         "yellow",
		"mdLink":            "cyan",
		"mdLinkUrl":         "muted",
		"mdCode":            "bright",
		"mdCodeBlock":       "fg",
		"mdCodeBlockBorder": "muted",
		"mdQuote":           "muted",
		"mdQuoteBorder":     "muted",
		"mdHr":              "muted",
		"mdListBullet":      "accent",
		"toolDiffAdded":     s.Git.Added,
		"toolDiffRemoved":   s.Git.Removed,
		"toolDiffContext":   "muted",
		"syntaxComment":     s.Syntax.Comment,
		"syntaxKeyword":     s.Syntax.Keyword,
		"syntaxFunction":    s.Syntax.Function,
		"syntaxString":      s.Syntax.String,
		"syntaxNumber":      s.Syntax.Number,
		"syntaxType":        s.Syntax.Type,
		"syntaxOperator":    s.Syntax.Operator,
		// Pi has no dedicated slots for variable/punctuation in the schema;
		// map to sensible reference-strings from vars.
		"syntaxVariable":    "cyan",
		"syntaxPunctuation": "muted",
		// Thinking ladder — dim off, warm up as level rises.
		"thinkingOff":     "surface1",
		"thinkingMinimal": "muted",
		"thinkingLow":     "blue",
		"thinkingMedium":  "accent",
		"thinkingHigh":    "bright",
		"thinkingXhigh":   "cyan",
		"thinkingText":    "muted",
		"bashMode":        "yellow",
	}

	pi := piThemeV4{
		Schema: "https://raw.githubusercontent.com/badlogic/pi-mono/main/packages/coding-agent/src/modes/interactive/theme/theme-schema.json",
		Name:   t.Name,
		Vars:   vars,
		Colors: colors,
	}

	// Allow hints.pi to inject or override vars.
	if h := t.Hint("pi"); h != nil {
		if extraVars, ok := h["vars"].(map[string]any); ok {
			for k, v := range extraVars {
				if s, ok := v.(string); ok {
					pi.Vars[k] = s
				}
			}
		}
		if extraColors, ok := h["colors"].(map[string]any); ok {
			for k, v := range extraColors {
				if s, ok := v.(string); ok {
					pi.Colors[k] = s
				}
			}
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")
	if err := enc.Encode(pi); err != nil {
		return fmt.Errorf("emit pi.json: %w", err)
	}

	// Overrides for pi.json come in as either raw JSON fragments (rare)
	// or a sidecar path. Rather than surgically merging JSON, append
	// nothing here; users write to hints.pi if they need to inject.
	return nil
}
