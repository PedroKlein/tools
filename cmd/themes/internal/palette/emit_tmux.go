package palette

import (
	"fmt"
	"io"
)

// v4 tmux emitter — reads theme.json, writes tmux.conf.
//
// TRANSPARENCY-FIRST DESIGN
// =========================
// Permanent tmux chrome uses `bg=default`. This is critical: Ghostty's
// `background-opacity-cells = true` policy applies opacity uniformly, but
// cells where tmux sets a hex background render as solid blocks against the
// translucent terminal bg.
//
// Visual differentiation for permanent chrome therefore uses:
//   * fg colors (accent for active window, muted for inactive, warning
//     for activity, error for bell, etc.)
//   * bold/italic attributes
//   * marker glyphs (▸ for active window, brackets for PREFIX/COPY badges)
//
// Copy-mode selection is different: it is transient functional UI, and it
// must stay readable while selecting text. We intentionally use explicit
// high-contrast backgrounds for mode-style and copy-mode search matches so
// the highlight does not collapse into a translucent pane background.
//
// See docs/plans/theme-transparency.md § Phase 3 for the full rationale
// behind keeping permanent tmux chrome transparent.

type tmuxEmitter struct{}

func (tmuxEmitter) App() string      { return "tmux" }
func (tmuxEmitter) Filename() string { return "tmux.conf" }

func (e tmuxEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitTmuxSemantic, NoHints)
}

func emitTmuxSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	accent := s.Accent
	fg := s.Fg
	muted := s.Muted
	border := s.Border
	borderActive := s.Accent2
	ok := s.Ok                // PREFIX badge fg
	cyan := t.Palette.Ansi[6] // COPY badge fg

	// --- status bar container ------------------------------------------
	fmt.Fprintf(w, "set -g status-style \"bg=default,fg=%s\"\n", fg)

	// --- window tabs ---------------------------------------------------
	// Inactive:  "  1 name "   (fg=muted, no marker)
	// Active:    "▸ 1 name "   (fg=accent, bold, marker glyph)
	fmt.Fprintf(w, "set -g window-status-style \"bg=default,fg=%s\"\n", muted)
	fmt.Fprintf(w, "set -g window-status-current-style \"bg=default,fg=%s,bold\"\n", accent)
	fmt.Fprintf(w, "set -g window-status-activity-style \"bg=default,fg=%s\"\n", s.Warning)
	fmt.Fprintf(w, "set -g window-status-bell-style \"bg=default,fg=%s,bold\"\n", s.Error)
	fmt.Fprintf(w, "set -g window-status-format \"#[fg=%s]  #I #W \"\n", muted)
	fmt.Fprintf(w, "set -g window-status-current-format \"#[fg=%s,bold]▸ #I #W#{?window_zoomed_flag,✴,} \"\n", accent)

	// --- pane borders --------------------------------------------------
	fmt.Fprintf(w, "set -g pane-border-style \"bg=default,fg=%s\"\n", border)
	fmt.Fprintf(w, "set -g pane-active-border-style \"bg=default,fg=%s\"\n", borderActive)

	// --- pane border footer (cwd + CPU/RAM/battery + time) ------------
	// Explicit bg=default in each format token, fg-only for differentiation.
	fmt.Fprintf(w,
		"set -g pane-border-format \""+
			"#[align=left,bg=default,fg=%s] #{pane_current_path} "+
			"#[align=right,bg=default,fg=%s]#(~/.config/tmux/plugins/tmux-cpu/scripts/ram_percentage.sh 2>/dev/null) RAM "+
			"#[bg=default,fg=%s]#(~/.config/tmux/plugins/tmux-cpu/scripts/cpu_percentage.sh 2>/dev/null) CPU "+
			"#[bg=default,fg=%s]#(~/.config/tmux/plugins/tmux-battery/scripts/battery_percentage.sh 2>/dev/null) "+
			"#[bg=default,fg=%s]%%H:%%M %%d-%%b \"\n",
		muted, cyan, accent, ok, muted)

	// --- window canvas (inside panes) ---------------------------------
	fmt.Fprintf(w, "set -g window-style \"bg=default,fg=%s\"\n", fg)
	fmt.Fprintf(w, "set -g window-active-style \"bg=default,fg=%s\"\n", fg)
	fmt.Fprintf(w, "set -g cursor-colour \"%s\"\n", s.Cursor)

	// --- messages / modes / copy ---------------------------------------
	// message-style is prompt/:command bar and status messages. Bold
	// warning fg on default bg keeps it visible without an opaque bar.
	fmt.Fprintf(w, "set -g message-style \"bg=default,fg=%s,bold\"\n", s.Warning)
	fmt.Fprintf(w, "set -g message-command-style \"bg=default,fg=%s,bold\"\n", accent)
	// mode-style and copy-mode matches are transient functional UI. Use
	// explicit high-contrast backgrounds here instead of reverse+default: in
	// translucent Ghostty panes, reverse+default can collapse into the pane bg
	// and make the selected text unreadable.
	fmt.Fprintf(w, "set -g mode-style \"bg=%s,fg=%s\"\n", accent, s.Bg)
	fmt.Fprintf(w, "set -g copy-mode-selection-style \"bg=%s,fg=%s\"\n", accent, s.Bg)
	fmt.Fprintf(w, "set -g clock-mode-colour \"%s\"\n", accent)
	fmt.Fprintf(w, "set -g copy-mode-match-style \"bg=%s,fg=%s\"\n", s.Warning, s.Bg)
	fmt.Fprintf(w, "set -g copy-mode-current-match-style \"bg=%s,fg=%s,bold\"\n", accent, s.Bg)

	// --- status-left: PREFIX / COPY badges + session name -------------
	// Badges are bracket-decorated bold fg-only. Previously colored pills
	// (bg=ok, bg=cyan); now fg-only for transparency. Session name gets
	// a leading marker and a trailing divider glyph.
	fmt.Fprintf(w,
		"set -g status-left \""+
			"#{?client_prefix,#[bg=default#,fg=%s#,bold] [PREFIX] #[default],}"+
			"#{?pane_in_mode,#[bg=default#,fg=%s#,bold] [COPY] #[default],}"+
			"#[bg=default,fg=%s,bold] #S #[bg=default,fg=%s]│ \"\n",
		ok, cyan, accent, muted)

	// --- status-right: cwd basename + divider + time ------------------
	fmt.Fprintf(w,
		"set -g status-right \""+
			"#[bg=default,fg=%s]#{b:pane_current_path} "+
			"#[bg=default,fg=%s]│ "+
			"#[bg=default,fg=%s]%%H:%%M \"\n",
		muted, accent, fg)

	return nil
}
