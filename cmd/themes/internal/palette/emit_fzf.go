package palette

import (
	"fmt"
	"io"
)

// v4 fzf emitter — reads theme.json, writes fzf.sh.
//
// Output is a shell script that exports FZF_DEFAULT_OPTS_COLORS built
// from semantic slots. The user's .zshrc sources fzf.sh and appends
// $FZF_DEFAULT_OPTS_COLORS to $FZF_DEFAULT_OPTS.

type fzfEmitter struct{}

func (fzfEmitter) App() string      { return "fzf" }
func (fzfEmitter) Filename() string { return "fzf.sh" }

func (e fzfEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitFzfSemantic, NoHints)
}

func emitFzfSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	// fzf's color spec is `--color=fg:HEX,bg:HEX,hl:HEX,...`. Full list:
	// https://github.com/junegunn/fzf/wiki/Color-schemes
	opts := fmt.Sprintf(
		"--color=fg:%s,bg:-1,hl:%s,fg+:%s,bg+:%s,hl+:%s,"+
			"info:%s,prompt:%s,pointer:%s,marker:%s,spinner:%s,header:%s,"+
			"border:%s",
		s.Fg,        // normal fg
		s.Accent,    // hl
		s.Fg,        // fg+
		s.SelectionBg, // bg+
		s.Accent2,   // hl+
		s.Muted,     // info
		s.Accent,    // prompt
		s.Accent2,   // pointer
		s.Warning,   // marker
		s.Accent,    // spinner
		s.Info,      // header
		s.Border,    // border
	)
	fmt.Fprintf(w, "export FZF_DEFAULT_OPTS_COLORS=%q\n", opts)
	return nil
}
