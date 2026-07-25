package palette

import (
	"fmt"
	"io"
)

// v4 opencode emitter — writes a bare filename to opencode.name.
//
// The opencode reload hook reads this file and runs jq against
// ~/.config/opencode/tui.json to set the theme.name field. No color
// data crosses this file — opencode carries its own themes internally.

type opencodeEmitter struct{}

func (opencodeEmitter) App() string      { return "opencode" }
func (opencodeEmitter) Filename() string { return "opencode.name" }

func (opencodeEmitter) Emit(t *Theme, w io.Writer) error {
	// Prefer hints.opencode.name if the theme wants to map to a
	// different opencode theme identifier (e.g. tokyonight → tokyo-night).
	name := t.Name
	if h := t.Hint("opencode"); h != nil {
		if v, ok := h["name"].(string); ok && v != "" {
			name = v
		}
	}
	_, err := fmt.Fprintln(w, name)
	return err
}
