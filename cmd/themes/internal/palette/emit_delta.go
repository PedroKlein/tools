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

	// Delta style syntax: `<fg> <bg>` where `normal` = use terminal's
	// normal foreground. Emitting `normal "#63B07A"` paints the *entire*
	// diff line background in full-strength green — too loud to read
	// against the text.
	//
	// Delta's own documented recommendation is subtle bg tints: blend
	// the diff hue with the theme's bg color so the bar is a faint wash,
	// not a slab of paint. Use ~12% blend for the full-line style and
	// ~30% for the emph-style (word-level highlights within the line).
	//
	// Foreground stays as `syntax` so treesitter highlighting bleeds
	// through cleanly on top of the tinted bg.
	minusBg := mix(s.Bg, s.Git.Removed, 0.12)
	minusBgEmph := mix(s.Bg, s.Git.Removed, 0.30)
	plusBg := mix(s.Bg, s.Git.Added, 0.12)
	plusBgEmph := mix(s.Bg, s.Git.Added, 0.30)

	fmt.Fprintln(w, "[delta]")
	fmt.Fprintf(w, "    minus-style = syntax %q\n", minusBg)
	fmt.Fprintf(w, "    minus-emph-style = syntax %q bold\n", minusBgEmph)
	fmt.Fprintf(w, "    plus-style = syntax %q\n", plusBg)
	fmt.Fprintf(w, "    plus-emph-style = syntax %q bold\n", plusBgEmph)
	fmt.Fprintf(w, "    file-style = %q bold\n", s.Accent)
	fmt.Fprintf(w, "    hunk-header-style = %q\n", s.Muted)
	fmt.Fprintf(w, "    hunk-header-decoration-style = %q box\n", s.Border)
	// Line-numbers stay full-saturation because they're single glyphs on
	// the gutter — they need to *pop* against the subtle line bg.
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
