package palette

import (
	"fmt"
	"io"
)

// v4 delta emitter — writes delta.gitconfig fragment.
//
// The user's .gitconfig has `[include] path = .../delta.gitconfig` so
// delta options here layer on top of whatever's in ~/.gitconfig.

type deltaEmitter struct{}

func (deltaEmitter) App() string      { return "delta" }
func (deltaEmitter) Filename() string { return "delta.gitconfig" }

func (e deltaEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitDeltaSemantic, NoHints)
}

func emitDeltaSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	fmt.Fprintln(w, "[delta]")
	fmt.Fprintf(w, "    minus-style = normal %q\n", s.Git.Removed)
	fmt.Fprintf(w, "    minus-emph-style = normal %q\n", s.Error)
	fmt.Fprintf(w, "    plus-style = normal %q\n", s.Git.Added)
	fmt.Fprintf(w, "    plus-emph-style = normal %q\n", s.Ok)
	fmt.Fprintf(w, "    file-style = %q bold\n", s.Accent)
	fmt.Fprintf(w, "    hunk-header-style = %q\n", s.Muted)
	fmt.Fprintf(w, "    hunk-header-decoration-style = %q box\n", s.Border)
	fmt.Fprintf(w, "    line-numbers-minus-style = %q\n", s.Git.Removed)
	fmt.Fprintf(w, "    line-numbers-plus-style = %q\n", s.Git.Added)
	fmt.Fprintf(w, "    line-numbers-zero-style = %q\n", s.Muted)
	fmt.Fprintf(w, "    zero-style = %q\n", s.Fg)
	fmt.Fprintf(w, "    commit-decoration-style = %q box\n", s.Accent)
	fmt.Fprintf(w, "    blame-code-style = syntax\n")
	// Quote the value: `#hex` starts a comment in git-config INI syntax,
	// so `blame-palette = #111C18 ...` parses as an empty value (silently!)
	// and delta panics with 'blame-palette must not be empty'. Wrapping
	// the whole value in double quotes turns it into a single string that
	// delta then splits on whitespace.
	fmt.Fprintf(w, "    blame-palette = \"%s %s %s %s\"\n", s.Bg, s.BgAlt, s.SelectionBg, s.Border)
	return nil
}
