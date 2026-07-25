package palette

import (
	"fmt"
	"io"
)

// v4 lazygit emitter — writes a YAML fragment with theme colors.

type lazygitEmitter struct{}

func (lazygitEmitter) App() string      { return "lazygit" }
func (lazygitEmitter) Filename() string { return "lazygit.yml" }

func (e lazygitEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitLazygitSemantic, NoHints)
}

func emitLazygitSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	// The baseline emits the `gui:` mapping at column 0. This block
	// writes `theme:` indented as a child of that mapping. Emitting
	// another `gui:` at column 0 here would produce YAML with two
	// top-level `gui:` keys, which lazygit rejects at load time:
	//   yaml: unmarshal errors: mapping key "gui" already defined
	fmt.Fprintln(w, "  theme:")
	fmt.Fprintf(w, "    activeBorderColor: [%q, bold]\n", s.Accent)
	fmt.Fprintf(w, "    inactiveBorderColor: [%q]\n", s.Border)
	fmt.Fprintf(w, "    optionsTextColor: [%q]\n", s.Info)
	fmt.Fprintf(w, "    selectedLineBgColor: [%q]\n", s.SelectionBg)
	fmt.Fprintf(w, "    cherryPickedCommitBgColor: [%q]\n", s.Warning)
	fmt.Fprintf(w, "    cherryPickedCommitFgColor: [%q]\n", s.Bg)
	fmt.Fprintf(w, "    unstagedChangesColor: [%q]\n", s.Error)
	fmt.Fprintf(w, "    defaultFgColor: [%q]\n", s.Fg)
	fmt.Fprintf(w, "    searchingActiveBorderColor: [%q]\n", s.Accent2)
	return nil
}
