package palette

import (
	"fmt"
	"io"
)

// EmitK9s writes a k9s skin using semantic roles for surfaces and accents.
func EmitK9s(w io.Writer, p *Palette) error {
	a := p.Alacritty
	accent := p.Var("accent", "green")
	bright := p.Var("bright", "b_green")
	muted := p.Role("muted", "b_black")
	borderMuted := p.Role("borderMuted", "b_black")
	borderAccent := p.Role("borderAccent", "b_green")
	selectedBg := p.Role("selectedBg", "b_black")

	fmt.Fprintf(w, `# k9s skin (derived; do not edit)
k9s:
  body:
    fgColor: "%s"
    bgColor: "%s"
    logoColor: "%s"
  prompt:
    fgColor: "%s"
    bgColor: "%s"
    suggestColor: "%s"
  info:
    fgColor: "%s"
    sectionColor: "%s"
  dialog:
    fgColor: "%s"
    bgColor: "%s"
    buttonFgColor: "%s"
    buttonBgColor: "%s"
    buttonFocusFgColor: "%s"
    buttonFocusBgColor: "%s"
    labelFgColor: "%s"
    fieldFgColor: "%s"
  frame:
    border:
      fgColor: "%s"
      focusColor: "%s"
    menu:
      fgColor: "%s"
      keyColor: "%s"
      numKeyColor: "%s"
    crumbs:
      fgColor: "%s"
      bgColor: "%s"
      activeColor: "%s"
    status:
      newColor: "%s"
      modifyColor: "%s"
      addColor: "%s"
      pendingColor: "%s"
      errorColor: "%s"
      highlightColor: "%s"
      killColor: "%s"
      completedColor: "%s"
    title:
      fgColor: "%s"
      bgColor: "%s"
      highlightColor: "%s"
      counterColor: "%s"
      filterColor: "%s"
  views:
    table:
      fgColor: "%s"
      bgColor: "%s"
      cursorFgColor: "%s"
      cursorBgColor: "%s"
      header:
        fgColor: "%s"
        bgColor: "%s"
        sorterColor: "%s"
    xray:
      fgColor: "%s"
      bgColor: "%s"
      cursorColor: "%s"
      graphicColor: "%s"
    yaml:
      keyColor: "%s"
      valueColor: "%s"
      colonColor: "%s"
    logs:
      fgColor: "%s"
      bgColor: "%s"
      indicator:
        fgColor: "%s"
        bgColor: "%s"
        toggleOnColor: "%s"
        toggleOffColor: "%s"
`,
		a.FG, a.BG, accent,             // body
		a.FG, a.BG, muted,              // prompt
		muted, a.FG,                    // info
		a.FG, selectedBg, a.BG, accent, // dialog fg/bg/btnFg/btnBg
		a.BG, bright, a.Yellow, a.FG,   // dialog btnFocus/label/field
		borderMuted, borderAccent,      // frame.border
		a.FG, accent, a.Cyan,           // menu fg/key/numKey
		a.FG, a.BG, accent,             // crumbs fg/bg/active
		bright, a.Yellow, bright, a.BCyan, a.Red, accent, a.Red, muted, // status
		a.FG, a.BG, a.Yellow, bright, a.Cyan,   // title
		a.FG, a.BG, a.BG, selectedBg,           // table body
		accent, a.BG, bright,                   // table header
		a.FG, a.BG, selectedBg, accent,         // xray
		accent, a.FG, muted,                    // yaml
		a.FG, a.BG,                             // logs
		accent, a.BG, bright, muted,            // logs.indicator
	)
	return nil
}

// EmitTelevision writes a television theme.toml.
func EmitTelevision(w io.Writer, p *Palette) error {
	a := p.Alacritty
	accent := p.Var("accent", "green")
	bright := p.Var("bright", "b_green")
	muted := p.Role("muted", "b_black")
	borderMuted := p.Role("borderMuted", "b_black")
	selectedBg := p.Role("selectedBg", "b_black")

	fmt.Fprintf(w, `# television theme (derived; do not edit)
background              = "%s"
border_fg               = "%s"
text_fg                 = "%s"
dimmed_text_fg          = "%s"

input_text_fg           = "%s"
result_count_fg         = "%s"

result_name_fg          = "%s"
result_line_number_fg   = "%s"
result_value_fg         = "%s"
selection_fg            = "%s"
selection_bg            = "%s"
match_fg                = "%s"

preview_title_fg        = "%s"

channel_mode_fg         = "%s"
channel_mode_bg         = "%s"
remote_control_mode_fg  = "%s"
remote_control_mode_bg  = "%s"
send_to_channel_mode_fg = "%s"
`,
		a.BG, borderMuted, a.FG, muted,
		bright, accent,
		bright, accent, a.FG, bright, selectedBg, accent,
		bright,
		a.BG, accent,
		a.BG, bright,
		a.FG,
	)
	return nil
}

// EmitLazygit writes a lazygit theme YAML.
func EmitLazygit(w io.Writer, p *Palette) error {
	a := p.Alacritty
	fmt.Fprintf(w, `# lazygit theme (derived; do not edit)
gui:
  theme:
    lightTheme: false
    activeBorderColor:
      - "%s"
      - bold
    inactiveBorderColor:
      - "%s"
    optionsTextColor:
      - "%s"
    selectedLineBgColor:
      - "%s"
    cherryPickedCommitBgColor:
      - "%s"
    cherryPickedCommitFgColor:
      - "%s"
    unstagedChangesColor:
      - "%s"
    defaultFgColor:
      - "%s"
`,
		p.Role("borderAccent", "b_green"),
		p.Role("borderMuted", "b_black"),
		a.BCyan,
		p.Role("selectedBg", "b_black"),
		p.Var("accent", "green"),
		a.BG,
		a.Red,
		a.FG,
	)
	return nil
}

// EmitGhDash writes a gh-dash theme YAML.
func EmitGhDash(w io.Writer, p *Palette) error {
	a := p.Alacritty
	fmt.Fprintf(w, `# gh-dash theme (derived; do not edit)
theme:
  colors:
    text:
      primary: "%s"
      secondary: "%s"
      inverted: "%s"
      faint: "%s"
      warning: "%s"
      success: "%s"
      error: "%s"
    background:
      selected: "%s"
    border:
      primary: "%s"
      secondary: "%s"
      faint: "%s"
`,
		a.FG, a.BCyan, a.BG,
		p.Role("muted", "b_black"),
		a.Yellow,
		p.Var("bright", "b_green"),
		a.Red,
		p.Role("selectedBg", "b_black"),
		p.Role("borderAccent", "b_green"),
		p.Role("border", "blue"),
		p.Role("borderMuted", "b_black"),
	)
	return nil
}

// EmitDelta writes a delta.gitconfig section.
func EmitDelta(w io.Writer, p *Palette) error {
	// toolErrorBg / toolSuccessBg roles have hardcoded default hex fallbacks
	// (not ANSI names) for the diff highlight surfaces — Python parity.
	minus := roleWithHexDefault(p, "toolErrorBg", "#241414")
	plus := roleWithHexDefault(p, "toolSuccessBg", "#142418")
	accent := p.Var("accent", "green")
	bright := p.Var("bright", "b_green")
	muted := p.Role("muted", "b_black")
	borderMuted := p.Role("borderMuted", "b_black")

	fmt.Fprintf(w, `; delta theme (derived; do not edit)
[delta]
    syntax-theme = current
    dark = true
    line-numbers = true
    side-by-side = false
    minus-style = syntax "%s"
    minus-emph-style = syntax bold "%s"
    plus-style = syntax "%s"
    plus-emph-style = syntax bold "%s"
    zero-style = syntax
    hunk-header-style = "%s" bold
    hunk-header-decoration-style = "%s" box
    file-style = "%s" bold
    file-decoration-style = "%s" ul
    commit-decoration-style = "%s" box
    line-numbers-left-style = "%s"
    line-numbers-right-style = "%s"
    line-numbers-minus-style = "%s"
    line-numbers-plus-style = "%s"
    line-numbers-zero-style = "%s"
`,
		minus, minus,
		plus, plus,
		accent, borderMuted,
		bright, borderMuted,
		accent,
		muted, muted, p.Alacritty.Red, bright, muted,
	)
	return nil
}

// EmitFzf writes a fzf export line for FZF_DEFAULT_OPTS.
func EmitFzf(w io.Writer, p *Palette) error {
	a := p.Alacritty
	fmt.Fprintf(w, `# fzf theme (derived; do not edit)
export FZF_DEFAULT_OPTS="
  --color=bg:%s,bg+:%s
  --color=fg:%s,fg+:%s
  --color=hl:%s,hl+:%s
  --color=info:%s,marker:%s
  --color=prompt:%s,spinner:%s
  --color=pointer:%s,header:%s
  --color=border:%s,label:%s
  --color=query:%s
"
`,
		a.BG, p.Role("selectedBg", "b_black"),
		a.FG, a.White,
		p.Var("bright", "b_green"), p.Var("bright", "b_green"),
		a.BCyan, p.Var("bright", "b_green"),
		p.Var("accent", "green"), a.Yellow,
		p.Var("bright", "b_green"), p.Role("muted", "b_black"),
		p.Role("border", "blue"), a.FG,
		a.FG,
	)
	return nil
}

// EmitZshHighlight writes a zsh-syntax-highlighting theme.
func EmitZshHighlight(w io.Writer, p *Palette) error {
	a := p.Alacritty
	accent := p.Var("accent", "green")
	bright := p.Var("bright", "b_green")

	fmt.Fprintf(w, `# zsh-syntax-highlighting (derived; do not edit)
typeset -gA ZSH_HIGHLIGHT_STYLES
ZSH_HIGHLIGHT_STYLES[default]='fg=%s'
ZSH_HIGHLIGHT_STYLES[comment]='fg=%s,italic'
ZSH_HIGHLIGHT_STYLES[reserved-word]='fg=%s'
ZSH_HIGHLIGHT_STYLES[builtin]='fg=%s'
ZSH_HIGHLIGHT_STYLES[function]='fg=%s'
ZSH_HIGHLIGHT_STYLES[command]='fg=%s'
ZSH_HIGHLIGHT_STYLES[alias]='fg=%s'
ZSH_HIGHLIGHT_STYLES[precommand]='fg=%s,underline'
ZSH_HIGHLIGHT_STYLES[hashed-command]='fg=%s'
ZSH_HIGHLIGHT_STYLES[path]='fg=%s,underline'
ZSH_HIGHLIGHT_STYLES[globbing]='fg=%s'
ZSH_HIGHLIGHT_STYLES[single-hyphen-option]='fg=%s'
ZSH_HIGHLIGHT_STYLES[double-hyphen-option]='fg=%s'
ZSH_HIGHLIGHT_STYLES[single-quoted-argument]='fg=%s'
ZSH_HIGHLIGHT_STYLES[double-quoted-argument]='fg=%s'
ZSH_HIGHLIGHT_STYLES[dollar-quoted-argument]='fg=%s'
ZSH_HIGHLIGHT_STYLES[unknown-token]='fg=%s'
ZSH_HIGHLIGHT_STYLES[assign]='fg=%s'
`,
		a.FG,
		p.Role("syntaxComment", "b_black"),
		p.Role("syntaxKeyword", "magenta"),
		accent,
		p.Role("syntaxFunction", "yellow"),
		bright, bright, bright, bright,
		a.BCyan,
		a.Yellow,
		a.BCyan, a.BCyan,
		p.Role("syntaxString", "b_green"),
		p.Role("syntaxString", "b_green"),
		p.Role("syntaxString", "b_green"),
		a.Red,
		p.Role("syntaxVariable", "b_cyan"),
	)
	return nil
}

// roleWithHexDefault is like Role() but the fallback is treated as a raw
// #hex color instead of an ANSI slot name. Used where the Python source
// hardcodes a hex fallback (e.g. delta's tool-error background).
func roleWithHexDefault(p *Palette, name, fallbackHex string) string {
	if raw, ok := p.Roles[name]; ok {
		if raw != "" && raw[0] == '#' {
			return normHex(raw)
		}
		if hex, ok := p.Vars[raw]; ok {
			return hex
		}
	}
	if hex, ok := p.Vars[name]; ok {
		return hex
	}
	return normHex(fallbackHex)
}
