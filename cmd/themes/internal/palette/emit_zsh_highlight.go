package palette

import (
	"fmt"
	"io"
)

// v4 zsh-syntax-highlighting emitter — writes ZSH_HIGHLIGHT_STYLES[<scope>]
// entries derived from semantic slots.

type zshHighlightEmitter struct{}

func (zshHighlightEmitter) App() string      { return "zsh-syntax-highlight" }
func (zshHighlightEmitter) Filename() string { return "zsh-syntax-highlight.zsh" }

func (e zshHighlightEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitZshHighlightSemantic, NoHints)
}

func emitZshHighlightSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	set := func(scope, color string) {
		fmt.Fprintf(w, "ZSH_HIGHLIGHT_STYLES[%s]=fg=%s\n", scope, color)
	}
	// Core scopes.
	set("comment", s.Muted)
	set("default", s.Fg)
	set("unknown-token", s.Error)
	set("reserved-word", s.Syntax.Keyword)
	set("alias", s.Syntax.Function)
	set("builtin", s.Syntax.Function)
	set("function", s.Syntax.Function)
	set("command", s.Syntax.Function)
	set("path", s.Info)
	set("globbing", s.Warning)
	set("history-expansion", s.Warning)
	set("single-quoted-argument", s.Syntax.String)
	set("double-quoted-argument", s.Syntax.String)
	set("dollar-quoted-argument", s.Syntax.String)
	set("assign", s.Accent2)
	set("named-fd", s.Info)
	set("numeric-fd", s.Syntax.Number)
	set("bracket-error", s.Error)
	// Bracket level rotation.
	set("bracket-level-1", s.Accent)
	set("bracket-level-2", s.Accent2)
	set("bracket-level-3", s.Info)
	set("bracket-level-4", s.Warning)
	set("cursor-matchingbracket", s.Ok)
	return nil
}
