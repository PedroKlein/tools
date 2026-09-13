package palette

import (
	"fmt"
	"io"
)

// v4 television emitter — writes a standalone custom theme for tv (television).

type televisionEmitter struct{}

func (televisionEmitter) App() string      { return "television" }
func (televisionEmitter) Filename() string { return "television.toml" }

func (e televisionEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitTelevisionSemantic, NoHints)
}

func emitTelevisionSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	fmt.Fprintf(w, "background = %q\n", s.Bg)
	fmt.Fprintf(w, "border_fg = %q\n", s.Border)
	fmt.Fprintf(w, "text_fg = %q\n", s.Fg)
	fmt.Fprintf(w, "dimmed_text_fg = %q\n", s.Muted)
	fmt.Fprintf(w, "input_text_fg = %q\n", s.Accent)
	fmt.Fprintf(w, "result_count_fg = %q\n", s.Muted)
	fmt.Fprintf(w, "result_name_fg = %q\n", s.Accent2)
	fmt.Fprintf(w, "result_line_number_fg = %q\n", s.Warning)
	fmt.Fprintf(w, "result_value_fg = %q\n", s.Fg)
	fmt.Fprintf(w, "selection_fg = %q\n", s.SelectionFg)
	fmt.Fprintf(w, "selection_bg = %q\n", s.SelectionBg)
	fmt.Fprintf(w, "match_fg = %q\n", s.Warning)
	fmt.Fprintf(w, "preview_title_fg = %q\n", s.Accent)
	fmt.Fprintf(w, "channel_mode_fg = %q\n", s.Bg)
	fmt.Fprintf(w, "channel_mode_bg = %q\n", s.Accent2)
	fmt.Fprintf(w, "remote_control_mode_fg = %q\n", s.Bg)
	fmt.Fprintf(w, "remote_control_mode_bg = %q\n", s.Ok)
	fmt.Fprintf(w, "action_picker_mode_fg = %q\n", s.Bg)
	fmt.Fprintf(w, "action_picker_mode_bg = %q\n", s.Accent)
	return nil
}
