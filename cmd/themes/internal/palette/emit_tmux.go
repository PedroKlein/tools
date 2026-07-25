package palette

import (
	"fmt"
	"io"
)

// v4 tmux emitter — reads theme.json, writes tmux.conf.
//
// Design: transparent status bar (bg=default, terminal bg shows through),
// accent-tinted current tab with YIQ-contrast foreground so it's
// readable on both light and dark themes.

type tmuxEmitter struct{}

func (tmuxEmitter) App() string      { return "tmux" }
func (tmuxEmitter) Filename() string { return "tmux.conf" }

func (e tmuxEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitTmuxSemantic, NoHints)
}

func emitTmuxSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	accent := s.Accent
	accentFG := YIQContrast(accent)
	fg := s.Fg
	muted := s.Muted
	border := s.Border
	borderActive := s.Accent2
	surface1 := s.SelectionBg
	ok := s.Ok        // battery / prefix-mode bg
	cyan := t.Palette.Ansi[6] // RAM / copy-mode bg

	// Status bar.
	fmt.Fprintf(w, "set -g status-style \"bg=default,fg=%s\"\n", fg)

	// Window tabs.
	fmt.Fprintf(w, "set -g window-status-style \"bg=default,fg=%s\"\n", muted)
	fmt.Fprintf(w, "set -g window-status-current-style \"bg=default,fg=%s,bold\"\n", accent)
	fmt.Fprintf(w, "set -g window-status-activity-style \"bg=default,fg=%s\"\n", s.Warning)
	fmt.Fprintf(w, "set -g window-status-bell-style \"bg=default,fg=%s,bold\"\n", s.Error)
	fmt.Fprintf(w, "set -g window-status-format \"#[fg=%s] #I #W \"\n", muted)
	fmt.Fprintf(w, "set -g window-status-current-format \"#[fg=%s,bg=%s,bold] #I #W#{?window_zoomed_flag,✴,} \"\n", accentFG, accent)

	// Pane borders.
	fmt.Fprintf(w, "set -g pane-border-style \"fg=%s\"\n", border)
	fmt.Fprintf(w, "set -g pane-active-border-style \"fg=%s\"\n", borderActive)

	// Pane border footer with cwd + CPU/RAM/battery + time (v3 parity).
	// References tmux-cpu and tmux-battery plugin scripts. Plugin
	// presence is guarded via 2>/dev/null so themes still work when
	// TPM hasn't installed them yet.
	fmt.Fprintf(w,
		"set -g pane-border-format \""+
			"#[align=left,fg=%s] #{pane_current_path} "+
			"#[align=right,fg=%s]#(~/.config/tmux/plugins/tmux-cpu/scripts/ram_percentage.sh 2>/dev/null) RAM "+
			"#[fg=%s]#(~/.config/tmux/plugins/tmux-cpu/scripts/cpu_percentage.sh 2>/dev/null) CPU "+
			"#[fg=%s]#(~/.config/tmux/plugins/tmux-battery/scripts/battery_percentage.sh 2>/dev/null) "+
			"#[fg=%s]%%H:%%M %%d-%%b \"\n",
		muted, cyan, accent, ok, muted)

	// Window bg (default so terminal bg shows through).
	fmt.Fprintf(w, "set -g window-style \"bg=default,fg=%s\"\n", fg)
	fmt.Fprintf(w, "set -g window-active-style \"bg=default,fg=%s\"\n", fg)
	fmt.Fprintf(w, "set -g cursor-colour \"%s\"\n", s.Cursor)

	// Messages / modes.
	fmt.Fprintf(w, "set -g message-style \"bg=%s,fg=%s\"\n", surface1, fg)
	fmt.Fprintf(w, "set -g message-command-style \"bg=%s,fg=%s\"\n", surface1, s.Warning)
	fmt.Fprintf(w, "set -g mode-style \"bg=%s,fg=%s\"\n", accent, s.Bg)
	fmt.Fprintf(w, "set -g clock-mode-colour \"%s\"\n", accent)
	fmt.Fprintf(w, "set -g copy-mode-match-style \"bg=%s,fg=%s\"\n", s.Warning, s.Bg)
	fmt.Fprintf(w, "set -g copy-mode-current-match-style \"bg=%s,fg=%s\"\n", accent, s.Bg)

	// Status-left: PREFIX indicator + COPY-mode indicator + session name.
	// V3 parity: bright green bg for PREFIX, cyan bg for COPY.
	fmt.Fprintf(w,
		"set -g status-left \""+
			"#{?client_prefix,#[fg=%s#,bg=%s#,bold] PREFIX #[default],}"+
			"#{?pane_in_mode,#[fg=%s#,bg=%s#,bold] COPY #[default],}"+
			"#[fg=%s,bold] #S #[fg=%s]│ \"\n",
		YIQContrast(ok), ok, YIQContrast(cyan), cyan, accent, muted)

	// Status-right: cwd basename + accent divider + time.
	fmt.Fprintf(w,
		"set -g status-right \""+
			"#[fg=%s]#{b:pane_current_path} "+
			"#[fg=%s]│ "+
			"#[fg=%s]%%H:%%M \"\n",
		muted, accent, fg)
	return nil
}
