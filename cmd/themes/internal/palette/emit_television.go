package palette

import (
	"fmt"
	"io"
)

// v4 television emitter — writes a TOML fragment for tv (television) TUI.

type televisionEmitter struct{}

func (televisionEmitter) App() string      { return "television" }
func (televisionEmitter) Filename() string { return "television.toml" }

func (e televisionEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitTelevisionSemantic, NoHints)
}

func emitTelevisionSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	// television.toml palette section.
	fmt.Fprintln(w, "[ui.theme]")
	fmt.Fprintf(w, "name = %q\n", t.Name)
	fmt.Fprintln(w, "[ui.theme.colors]")
	// background: empty string signals tv to use terminal default (transparent
	// through Ghostty's background-opacity). Emitting s.Bg (hex) would paint
	// a solid rectangle inside the tv popup.
	fmt.Fprintf(w, "background = %q\n", "")
	fmt.Fprintf(w, "foreground = %q\n", s.Fg)
	fmt.Fprintf(w, "muted = %q\n", s.Muted)
	fmt.Fprintf(w, "accent = %q\n", s.Accent)
	fmt.Fprintf(w, "accent_alt = %q\n", s.Accent2)
	fmt.Fprintf(w, "border = %q\n", s.Border)
	fmt.Fprintf(w, "highlight = %q\n", s.Accent)
	// selection_bg is the highlight for the currently-selected candidate
	// row — functional selection UI, kept opaque (same trade-off as k9s
	// cursorBg, btop selected_bg).
	fmt.Fprintf(w, "selection_bg = %q\n", s.SelectionBg)
	fmt.Fprintf(w, "match = %q\n", s.Warning)
	fmt.Fprintf(w, "error = %q\n", s.Error)
	fmt.Fprintf(w, "warning = %q\n", s.Warning)
	fmt.Fprintf(w, "ok = %q\n", s.Ok)
	return nil
}
