package palette

import (
	"fmt"
	"io"
)

// v4 aerospace emitter — reads theme.json, writes aerospace.toml.
//
// aerospace uses a partial TOML fragment that the user's aerospace.toml
// sources (via toml-include or manual copy-paste of the [gaps] and
// border color lines).
//
// Border colors are borrowed from palette.semantic.border (inactive)
// and palette.semantic.accent (focused). Gap sizes come from
// hints.aerospace.gaps.{inner,outer} when set.

type aerospaceEmitter struct{}

func (aerospaceEmitter) App() string      { return "aerospace" }
func (aerospaceEmitter) Filename() string { return "aerospace.toml" }

func (e aerospaceEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitAerospaceSemantic, emitAerospaceHints)
}

func emitAerospaceSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	// aerospace has no first-class border color option today, but users
	// commonly wire the FocusedWorkspace/Border colors through
	// sketchybar or JankyBorders. We emit them here as TOML for other
	// tools to consume by convention.
	fmt.Fprintln(w, "[colors]")
	fmt.Fprintf(w, "focused_border = \"%s\"\n", s.Accent)
	fmt.Fprintf(w, "inactive_border = \"%s\"\n", s.Border)
	fmt.Fprintf(w, "accent = \"%s\"\n", s.Accent)
	fmt.Fprintf(w, "bg = \"%s\"\n", s.Bg)
	return nil
}

func emitAerospaceHints(t *Theme, w io.Writer) error {
	h := t.Hint("aerospace")
	if h == nil {
		return nil
	}
	gapsRaw, ok := h["gaps"].(map[string]any)
	if !ok {
		return nil
	}
	fmt.Fprintln(w, "[gaps]")
	if v, ok := numericHint(gapsRaw, "inner"); ok {
		fmt.Fprintf(w, "inner.horizontal = %d\n", int(v))
		fmt.Fprintf(w, "inner.vertical   = %d\n", int(v))
	}
	if v, ok := numericHint(gapsRaw, "outer"); ok {
		fmt.Fprintf(w, "outer.left       = %d\n", int(v))
		fmt.Fprintf(w, "outer.bottom     = %d\n", int(v))
		fmt.Fprintf(w, "outer.top        = %d\n", int(v))
		fmt.Fprintf(w, "outer.right      = %d\n", int(v))
	}
	return nil
}
