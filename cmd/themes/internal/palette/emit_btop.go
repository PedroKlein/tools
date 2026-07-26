package palette

import (
	"fmt"
	"io"
)

// v4 btop emitter — writes btop.theme (a shell-like theme[<key>]=<hex>
// format). Uses palette.gradients.* when present.

type btopEmitter struct{}

func (btopEmitter) App() string      { return "btop" }
func (btopEmitter) Filename() string { return "btop.theme" }

func (btopEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, "btop", "#", emitBtopSemantic, NoHints)
}

func emitBtopSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	g := t.Palette.Gradients

	line := func(key, value string) {
		fmt.Fprintf(w, "theme[%s]=%q\n", key, value)
	}

	// Bar surfaces.
	// main_bg: intentionally empty string → btop reads this as "use
	// terminal default background", which lets Ghostty's translucent bg
	// show through. Emitting s.Bg (hex) here would override the baseline's
	// transparent default and paint a solid rectangle.
	line("main_bg", "")
	line("main_fg", s.Fg)
	line("title", s.Fg)
	line("hi_fg", s.Accent)
	// selected_bg is functional UI (highlights the currently-selected
	// process row) — kept opaque like k9s cursorBgColor and delta +/-
	// diff lines. Documented trade-off, see docs/plans/theme-transparency.md.
	line("selected_bg", s.SelectionBg)
	line("selected_fg", s.Fg)
	line("inactive_fg", s.Muted)
	line("graph_text", s.Fg)
	line("meter_bg", s.Border)

	// Process list.
	line("proc_misc", s.Info)
	line("cpu_box", s.Border)
	line("mem_box", s.Border)
	line("net_box", s.Border)
	line("proc_box", s.Border)
	line("div_line", s.Border)

	// Gradients (3-color tuples).
	line("temp_start", g.Temp[0])
	line("temp_mid", g.Temp[1])
	line("temp_end", g.Temp[2])
	line("cpu_start", g.Cpu[0])
	line("cpu_mid", g.Cpu[1])
	line("cpu_end", g.Cpu[2])
	line("mem_start", g.Memory[0])
	line("mem_mid", g.Memory[1])
	line("mem_end", g.Memory[2])
	line("net_start", g.Network[0])
	line("net_mid", g.Network[1])
	line("net_end", g.Network[2])

	// Process states.
	line("process_start", s.Ok)
	line("process_mid", s.Warning)
	line("process_end", s.Error)
	return nil
}
