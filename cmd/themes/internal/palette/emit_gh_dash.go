package palette

import (
	"fmt"
	"io"
)

// v4 gh-dash emitter — writes a YAML fragment with theme colors.

type ghDashEmitter struct{}

func (ghDashEmitter) App() string      { return "gh-dash" }
func (ghDashEmitter) Filename() string { return "gh-dash.yml" }

func (e ghDashEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitGhDashSemantic, NoHints)
}

func emitGhDashSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	fmt.Fprintln(w, "theme:")
	fmt.Fprintln(w, "  ui:")
	fmt.Fprintln(w, "    table:")
	fmt.Fprintf(w, "      selectedBackground: %q\n", s.SelectionBg)
	fmt.Fprintf(w, "      selectedForeground: %q\n", s.Fg)
	fmt.Fprintln(w, "  colors:")
	fmt.Fprintln(w, "    text:")
	fmt.Fprintf(w, "      primary: %q\n", s.Fg)
	fmt.Fprintf(w, "      secondary: %q\n", s.Muted)
	fmt.Fprintf(w, "      inverted: %q\n", s.Bg)
	fmt.Fprintf(w, "      faint: %q\n", s.FgDim)
	fmt.Fprintf(w, "      warning: %q\n", s.Warning)
	fmt.Fprintf(w, "      success: %q\n", s.Ok)
	fmt.Fprintf(w, "      error: %q\n", s.Error)
	fmt.Fprintln(w, "    background:")
	fmt.Fprintf(w, "      selected: %q\n", s.SelectionBg)
	fmt.Fprintln(w, "    border:")
	fmt.Fprintf(w, "      primary: %q\n", s.Accent)
	fmt.Fprintf(w, "      secondary: %q\n", s.Border)
	fmt.Fprintf(w, "      faint: %q\n", s.Muted)
	return nil
}
