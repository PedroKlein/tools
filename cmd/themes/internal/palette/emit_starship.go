package palette

import (
	"fmt"
	"io"
)

// v4 starship emitter — reads theme.json, writes starship.toml.
//
// Emits a named palette table indexed by theme name and sets
// `palette = "<name>"` at the top level so starship picks it up.

type starshipEmitter struct{}

func (starshipEmitter) App() string      { return "starship" }
func (starshipEmitter) Filename() string { return "starship.toml" }

func (e starshipEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitStarshipSemantic, NoHints)
}

func emitStarshipSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	a := t.Palette.Ansi
	fmt.Fprintf(w, "palette = %q\n\n", t.Name)
	fmt.Fprintf(w, "[palettes.%s]\n", t.Name)
	fmt.Fprintf(w, "bg      = %q\n", s.Bg)
	fmt.Fprintf(w, "fg      = %q\n", s.Fg)
	fmt.Fprintf(w, "accent  = %q\n", s.Accent)
	fmt.Fprintf(w, "accent2 = %q\n", s.Accent2)
	fmt.Fprintf(w, "muted   = %q\n", s.Muted)
	fmt.Fprintf(w, "error   = %q\n", s.Error)
	fmt.Fprintf(w, "warning = %q\n", s.Warning)
	fmt.Fprintf(w, "ok      = %q\n", s.Ok)
	fmt.Fprintf(w, "info    = %q\n", s.Info)
	fmt.Fprintf(w, "red     = %q\n", a[1])
	fmt.Fprintf(w, "green   = %q\n", a[2])
	fmt.Fprintf(w, "yellow  = %q\n", a[3])
	fmt.Fprintf(w, "blue    = %q\n", a[4])
	fmt.Fprintf(w, "magenta = %q\n", a[5])
	fmt.Fprintf(w, "cyan    = %q\n", a[6])
	fmt.Fprintf(w, "white   = %q\n", a[7])
	return nil
}
