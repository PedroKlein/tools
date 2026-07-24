package palette

import (
	"fmt"
	"io"
	"strings"
)

// EmitSketchybar writes a sketchybar palette shell file. Sketchybar wants
// 0xAARRGGBB colors, so we prepend an alpha byte to lowercased hex.
func EmitSketchybar(w io.Writer, p *Palette) error {
	a := p.Alacritty
	sb := func(hex, alpha string) string {
		return "0x" + alpha + strings.ToLower(strings.TrimPrefix(hex, "#"))
	}
	sbFull := func(hex string) string { return sb(hex, "ff") }

	// Semi-transparent bar bg (matches the previous handwritten colors.sh
	// 0xd0 alpha) and a lighter alpha for translucent surfaces.
	barBg := "d0"
	surfaceAlpha := "44"

	accent := p.Var("accent", "green")
	bright := p.Var("bright", "b_green")
	muted := p.Role("muted", "b_black")
	borderMuted := p.Role("borderMuted", "b_black")
	teal := p.Var("teal", "blue")
	jade := p.Var("jade", "green")

	fmt.Fprintf(w, `#!/usr/bin/env bash
# sketchybar palette (derived; do not edit)
# Variable names match the ones sketchybarrc + plugins reference.

# --- bar surface ------------------------------------------------------------
export BAR_BG=%s
export BAR_BORDER=%s

# --- foreground -------------------------------------------------------------
export FG=%s
export FG_MUTED=%s
export FG_BRIGHT=%s

# --- accents ---------------------------------------------------------------
export ACCENT=%s
export ACCENT_BRIGHT=%s
export HIGHLIGHT=%s

# --- semantic colors -------------------------------------------------------
export RED=%s
export YELLOW=%s
export CYAN=%s
export TEAL=%s
export JADE=%s
export MAGENTA=%s

# --- surfaces ---------------------------------------------------------------
export SURFACE=%s
export SURFACE_LIGHT=%s

# --- status semantics (used by battery/charging plugins) --------------------
export ICON=%s
export CHARGING=%s
export FOCUSED=%s
export FOCUSED_WORKSPACE=%s
export NON_EMPTY=%s
export BADGE=%s
export INFO=%s
export VOLUME=%s
export PERCENTAGE=%s
`,
		sb(a.BG, barBg), sbFull(borderMuted),
		sbFull(a.FG), sbFull(muted), sbFull(a.White),
		sbFull(accent), sbFull(bright), sbFull(accent),
		sbFull(a.Red), sbFull(a.Yellow), sbFull(a.Cyan),
		sbFull(teal), sbFull(jade), sbFull(a.Magenta),
		sbFull(borderMuted), sb(borderMuted, surfaceAlpha),
		sbFull(a.FG), sbFull(bright), sbFull(bright), sbFull(accent),
		sbFull(a.FG), sbFull(a.Yellow), sbFull(a.Cyan), sbFull(a.Cyan), sbFull(a.FG),
	)
	return nil
}

// batScope is one row in the bat.tmTheme XML — a syntax scope with the
// role that colors it and a fallback ANSI slot. The italic flag applies
// only to comments.
type batScope struct {
	Name    string
	Scope   string
	Role    string
	ANSI    string
	Italic  bool
}

var batScopes = []batScope{
	{"Comment", "comment", "syntaxComment", "b_black", true},
	{"Keyword", "keyword, storage.type, storage.modifier", "syntaxKeyword", "magenta", false},
	{"String", "string, string.quoted", "syntaxString", "b_green", false},
	{"Number", "constant.numeric, constant.language", "syntaxNumber", "cyan", false},
	{"Function", "entity.name.function, support.function", "syntaxFunction", "yellow", false},
	{"Variable", "variable, variable.other", "syntaxVariable", "b_cyan", false},
	{"Type", "entity.name.type, support.type", "syntaxType", "green", false},
	{"Operator", "keyword.operator", "syntaxOperator", "fg", false},
	{"Punctuation", "punctuation", "syntaxPunctuation", "b_black", false},
}

// EmitBat writes a bat.tmTheme plist. The format is Apple's XML property
// list; keep the indentation exactly the way Python emitted it (4-space
// nested dicts) to preserve golden-file byte parity.
func EmitBat(w io.Writer, p *Palette) error {
	a := p.Alacritty
	sel := p.Role("selectedBg", "b_black")

	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>name</key>
    <string>current</string>
    <key>settings</key>
    <array>
        <dict>
            <key>settings</key>
            <dict>
                <key>background</key>
                <string>%s</string>
                <key>foreground</key>
                <string>%s</string>
                <key>caret</key>
                <string>%s</string>
                <key>lineHighlight</key>
                <string>%s</string>
                <key>selection</key>
                <string>%s</string>
            </dict>
        </dict>
`,
		a.BG, a.FG, a.Cursor, sel, sel,
	)

	for _, s := range batScopes {
		fg := p.Role(s.Role, s.ANSI)
		italic := ""
		if s.Italic {
			italic = "\n                <key>fontStyle</key>\n                <string>italic</string>"
		}
		fmt.Fprintf(w, `        <dict>
            <key>name</key>
            <string>%s</string>
            <key>scope</key>
            <string>%s</string>
            <key>settings</key>
            <dict>
                <key>foreground</key>
                <string>%s</string>%s
            </dict>
        </dict>
`, s.Name, s.Scope, fg, italic)
	}

	fmt.Fprintf(w, `    </array>
    <key>uuid</key>
    <string>theme-%s</string>
</dict>
</plist>
`, p.Name)
	return nil
}
