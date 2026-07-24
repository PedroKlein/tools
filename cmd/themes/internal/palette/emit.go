package palette

import (
	"fmt"
	"io"
	"strings"
)

// EmitFunc renders one derived file for a theme.
type EmitFunc func(w io.Writer, p *Palette) error

// Emitter is one entry in the derive pipeline.
type Emitter struct {
	// App is a short label for logs/tests (e.g. "ghostty").
	App string
	// Filename is the target file basename inside the theme dir.
	Filename string
	// Emit renders content into w.
	Emit EmitFunc
}

// Emitters is the ordered list of all 13 derived files. Order matches the
// original Python theme-derive and is preserved to keep any log output
// stable across the Python→Go migration.
var Emitters = []Emitter{
	{"ghostty", "ghostty.conf", EmitGhostty},
	{"tmux", "tmux.conf", EmitTmux},
	{"starship", "starship.toml", EmitStarship},
	{"k9s", "k9s.yaml", EmitK9s},
	{"television", "television.toml", EmitTelevision},
	{"lazygit", "lazygit.yml", EmitLazygit},
	{"gh-dash", "gh-dash.yml", EmitGhDash},
	{"opencode", "opencode.name", EmitOpencode},
	{"bat", "bat.tmTheme", EmitBat},
	{"delta", "delta.gitconfig", EmitDelta},
	{"fzf", "fzf.sh", EmitFzf},
	{"zsh-highlight", "zsh-syntax-highlight.zsh", EmitZshHighlight},
	{"sketchybar", "sketchybar.sh", EmitSketchybar},
	{"pi", "pi.json", EmitPi},
	{"macos", ".macos.json", EmitMacOS},
}

// --- ghostty ---------------------------------------------------------------

// EmitGhostty writes ghostty.conf with 16 ANSI colors, primary, cursor,
// and theme-driven translucency (opacity+blur).
//
// Translucency defaults:
//   - Dark themes  (YIQ ≤ 128): opacity 0.85, blur 20  (softer wash)
//   - Light themes (YIQ >  128): opacity 0.97, blur 8   (readable over
//     bright wallpapers)
//
// Overridable via `[meta]` in palette.toml:
//
//	[meta]
//	opacity = 0.9
//	blur    = 15
//
// The `# primary` first-line marker is a load-bearing sentinel: derive()
// skips regenerating ghostty.conf if the first non-empty line isn't this
// marker, preserving upstream-authored ghostty.conf files verbatim.
func EmitGhostty(w io.Writer, p *Palette) error {
	a := p.Alacritty
	defaultOp := 0.85
	defaultBlur := 20
	if IsLight(a.BG) {
		defaultOp = 0.97
		defaultBlur = 8
	}
	opacity := p.MetaFloat("opacity", defaultOp)
	blur := p.MetaInt("blur", defaultBlur)

	fmt.Fprintf(w, "# primary\nbackground = %s\nforeground = %s\ncursor-color = %s\ncursor-text = %s\n\n",
		a.BG, a.FG, a.Cursor, a.CursorText)
	fmt.Fprintf(w, "# translucency (theme-driven; overridable via palette.toml [meta])\nbackground-opacity = %s\nbackground-blur = %d\n\n",
		trimFloat(opacity), blur)
	fmt.Fprintln(w, "# normal colors")
	for i := 0; i < 8; i++ {
		fmt.Fprintf(w, "palette = %d=%s\n", i, a.AnsiN(i))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "# bright colors")
	for i := 8; i < 16; i++ {
		fmt.Fprintf(w, "palette = %d=%s\n", i, a.AnsiN(i))
	}
	return nil
}

// trimFloat formats a float using Python's default repr rules (which is
// what the current on-disk output was written with): trailing zeros
// stripped, but at least one decimal for whole numbers.
//
// 0.85 → "0.85", 1.0 → "1.0", 0.5 → "0.5", 0.97 → "0.97".
func trimFloat(f float64) string {
	s := fmt.Sprintf("%g", f)
	// %g may render 1.0 as "1" — Python's repr renders "1.0". Match that.
	if !strings.Contains(s, ".") && !strings.Contains(s, "e") {
		s += ".0"
	}
	return s
}

// --- tmux ------------------------------------------------------------------

// EmitTmux writes a transparent status bar with an accent-tinted current
// tab and YIQ-computed foreground for readability across light/dark themes.
func EmitTmux(w io.Writer, p *Palette) error {
	a := p.Alacritty
	accent := p.Var("accent", "green")
	bright := p.Var("bright", "b_green")
	muted := p.Role("muted", "b_black")
	border := p.Role("borderMuted", "b_black")
	borderActive := p.Role("borderAccent", "b_green")
	surface1 := p.Var("surface1", p.Role("selectedBg", "b_black"))
	fg := a.FG
	accentFG := YIQContrast(accent)

	fmt.Fprintf(w, `# tmux theme (derived by theme-derive; do not edit)
#
# Transparent status bar (bg=default — terminal bg shows through).
# Current tab keeps an accent bg + YIQ-contrast fg so it's readable on
# both light and dark themes.

# --- status bar shell ------------------------------------------------------
set -g status-position top
set -g status-interval 5
set -g status-left-length 60
set -g status-right-length 100
set -g status-style "bg=default,fg=%s"

# --- window tabs -----------------------------------------------------------
# Inactive tabs: transparent, muted foreground.
# Current tab:   accent background + high-contrast foreground.
set -g window-status-separator " "
set -g window-status-style "bg=default,fg=%s"
set -g window-status-current-style "bg=default,fg=%s,bold"
set -g window-status-activity-style "bg=default,fg=%s"
set -g window-status-bell-style "bg=default,fg=%s,bold"

set -g window-status-format "#[fg=%s] #I #W "
set -g window-status-current-format "#[fg=%s,bg=%s,bold] #I #W#{?window_zoomed_flag,✴,} "

# --- panes -----------------------------------------------------------------
set -g pane-border-style "fg=%s"
set -g pane-active-border-style "fg=%s"
set -g pane-border-status bottom
set -g pane-border-format "#[align=left,fg=%s] #{pane_current_path} #[align=right,fg=%s]#(~/.config/tmux/plugins/tmux-cpu/scripts/ram_percentage.sh 2>/dev/null) RAM #[fg=%s]#(~/.config/tmux/plugins/tmux-cpu/scripts/cpu_percentage.sh 2>/dev/null) CPU #[fg=%s]#(~/.config/tmux/plugins/tmux-battery/scripts/battery_percentage.sh 2>/dev/null) #[fg=%s]%%H:%%M %%d-%%b "

# --- pane background: default (transparent, terminal bg shows through) ----
# Explicitly 'default' so tmux doesn't fill panes with its status bg.
set -g window-style "bg=default,fg=%s"
set -g window-active-style "bg=default,fg=%s"
set -g cursor-colour "%s"

# --- messages / modes ------------------------------------------------------
set -g message-style "bg=%s,fg=%s"
set -g message-command-style "bg=%s,fg=%s"
set -g mode-style "bg=%s,fg=%s"
set -g clock-mode-colour "%s"
set -g copy-mode-match-style "bg=%s,fg=%s"
set -g copy-mode-current-match-style "bg=%s,fg=%s"

# --- status-left / status-right --------------------------------------------
# Left: PREFIX / COPY indicators + session name.
# Right: cwd basename + accent divider + time.
set -g status-left "#{?client_prefix,#[fg=%s#,bg=%s#,bold] PREFIX #[default],}#{?pane_in_mode,#[fg=%s#,bg=%s#,bold] COPY #[default],}#[fg=%s,bold] #S #[fg=%s]│ "
set -g status-right "#[fg=%s]#{b:pane_current_path} #[fg=%s]│ #[fg=%s]%%H:%%M "
`,
		fg,                                     // status-style
		muted, accent, a.Yellow, a.Red,         // window-status-style, current, activity, bell
		muted, accentFG, accent,                // window-status-format + current-format
		border, borderActive,                   // pane-border-style
		muted, a.Cyan, accent, a.Yellow, muted, // pane-border-format: cwd, RAM, CPU, battery, time
		fg, fg, accent,                         // window-style, window-active-style, cursor-colour
		surface1, fg,                           // message-style
		surface1, a.Yellow,                     // message-command-style
		bright, a.BG,                           // mode-style
		bright,                                 // clock-mode-colour
		a.Yellow, a.BG,                         // copy-mode-match-style
		bright, a.BG,                           // copy-mode-current-match-style
		accentFG, a.Yellow, accentFG, a.Cyan,   // status-left PREFIX + COPY
		accent, muted,                          // status-left session
		muted, accent, fg,                      // status-right cwd + divider + time
	)
	return nil
}

// --- starship --------------------------------------------------------------

// EmitStarship writes a starship palette named after the theme.
func EmitStarship(w io.Writer, p *Palette) error {
	a := p.Alacritty
	fmt.Fprintf(w, `# starship palette (derived; do not edit)
palette = "%s"

[palettes.%s]
bg      = "%s"
fg      = "%s"
accent  = "%s"
bright  = "%s"
muted   = "%s"
red     = "%s"
green   = "%s"
yellow  = "%s"
blue    = "%s"
magenta = "%s"
cyan    = "%s"
white   = "%s"
`,
		p.Name, p.Name,
		a.BG, a.FG,
		p.Var("accent", "green"),
		p.Var("bright", "b_green"),
		p.Role("muted", "b_black"),
		a.Red, a.Green, a.Yellow, a.Blue, a.Magenta, a.Cyan, a.White,
	)
	return nil
}

// --- opencode --------------------------------------------------------------

// EmitOpencode writes the theme name (opencode reads a bare name file).
func EmitOpencode(w io.Writer, p *Palette) error {
	fmt.Fprintln(w, p.Name)
	return nil
}
