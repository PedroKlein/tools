package palette

import (
	"fmt"
	"io"
)

// v4 tuicr emitter — writes a flat TOML local theme for tuicr.
//
// tuicr local themes live in ~/.config/tuicr/themes/<name>.toml and use
// top-level color keys, matching examples/tuicr-teal.toml upstream.
type tuicrEmitter struct{}

func (tuicrEmitter) App() string      { return "tuicr" }
func (tuicrEmitter) Filename() string { return "tuicr.toml" }

func (e tuicrEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitTuicrSemantic, NoHints)
}

func emitTuicrSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic

	diffAddBg := blendHex(s.Bg, s.Git.Added, 22)
	diffDelBg := blendHex(s.Bg, s.Git.Removed, 22)
	modeFg := YIQContrast(s.Accent)
	infoFg := YIQContrast(s.Info)
	warningFg := YIQContrast(s.Warning)
	errorFg := YIQContrast(s.Error)

	line := func(key, value string) {
		fmt.Fprintf(w, "%s = %q\n", key, value)
	}

	line("panel_bg", s.BgAlt)
	line("bg_highlight", s.SelectionBg)
	line("fg_primary", s.Fg)
	line("fg_secondary", s.Muted)
	line("fg_dim", s.FgDim)
	fmt.Fprintln(w)

	line("diff_add", s.Git.Added)
	line("diff_add_bg", diffAddBg)
	line("diff_del", s.Git.Removed)
	line("diff_del_bg", diffDelBg)
	line("diff_context", s.Fg)
	line("diff_hunk_header", s.Info)
	line("expanded_context_fg", s.FgDim)
	fmt.Fprintln(w)

	line("syntax_add_bg", diffAddBg)
	line("syntax_del_bg", diffDelBg)
	line("syntax_theme", "current.tmTheme")
	fmt.Fprintln(w)

	line("file_added", s.Git.Added)
	line("file_modified", s.Git.Modified)
	line("file_deleted", s.Git.Removed)
	line("file_renamed", s.Syntax.Number)
	fmt.Fprintln(w)

	line("reviewed", s.Ok)
	line("pending", s.Warning)
	fmt.Fprintln(w)

	line("comment_note", s.Info)
	line("comment_suggestion", s.Accent)
	line("comment_issue", s.Error)
	line("comment_praise", s.Ok)
	fmt.Fprintln(w)

	line("border_focused", s.Accent)
	line("border_unfocused", s.Border)
	line("status_bar_bg", s.BgAlt)
	line("cursor_color", s.Cursor)
	line("cursor_line_bg", s.SelectionBg)
	line("branch_name", s.Syntax.Number)
	line("help_indicator", s.Muted)
	fmt.Fprintln(w)

	line("message_info_fg", infoFg)
	line("message_info_bg", s.Info)
	line("message_warning_fg", warningFg)
	line("message_warning_bg", s.Warning)
	line("message_error_fg", errorFg)
	line("message_error_bg", s.Error)
	line("update_badge_fg", warningFg)
	line("update_badge_bg", s.Warning)
	fmt.Fprintln(w)

	line("mode_fg", modeFg)
	line("mode_bg", s.Accent)
	return nil
}

func blendHex(base, overlay string, percent int) string {
	if percent <= 0 {
		return base
	}
	if percent >= 100 {
		return overlay
	}
	br, bg, bb := hexToRGB(base)
	or, og, ob := hexToRGB(overlay)
	mix := func(a, b int) int { return (a*(100-percent) + b*percent) / 100 }
	return fmt.Sprintf("#%02X%02X%02X", mix(br, or), mix(bg, og), mix(bb, ob))
}
