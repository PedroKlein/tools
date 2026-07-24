package palette

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Alacritty is the parsed alacritty.toml — 16 ANSI colors + primary + cursor.
//
// The struct exposes:
//   - Primary bg/fg
//   - Cursor cursor/text
//   - ANSI 0-7 (normal.black..white)
//   - ANSI 8-15 (bright.black..white)
//
// Accessed via named getters (BG, FG, Green, BGreen…) or the numeric
// AnsiN(idx) helper.
//
// get(name) is used internally by Palette.Role/Var to resolve string
// fallbacks like "green" or "b_green" against the parsed alacritty data.
type Alacritty struct {
	BG         string
	FG         string
	Cursor     string
	CursorText string

	// ANSI 0-7 (normal).
	Black, Red, Green, Yellow, Blue, Magenta, Cyan, White string
	// ANSI 8-15 (bright).
	BBlack, BRed, BGreen, BYellow, BBlue, BMagenta, BCyan, BWhite string
}

// Default ANSI colors when a slot is missing from alacritty.toml. Match
// the Python theme-derive fallbacks so ports are byte-identical.
var (
	ansiNames = [16]string{
		"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
		"b_black", "b_red", "b_green", "b_yellow", "b_blue", "b_magenta", "b_cyan", "b_white",
	}
	ansiDefaults = [16]string{
		"#000000", "#CD0000", "#00CD00", "#CDCD00", "#0000EE", "#CD00CD", "#00CDCD", "#E5E5E5",
		"#7F7F7F", "#FF0000", "#00FF00", "#FFFF00", "#5C5CFF", "#FF00FF", "#00FFFF", "#FFFFFF",
	}
)

// ParseAlacritty reads path (alacritty.toml) and returns the parsed colors.
//
// Bespoke minimal parser. alacritty.toml is small (~30 lines) and always
// has a rigid shape: [colors.primary], [colors.normal], [colors.bright],
// [colors.cursor]. A full TOML parser would be a heavy dep for zero gain.
//
// Missing keys fall back to the corresponding entry in ansiDefaults.
func ParseAlacritty(path string) (*Alacritty, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("alacritty.toml: %w", err)
	}
	defer f.Close()

	// section → key → raw value.
	sections := map[string]map[string]string{}
	current := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			s := strings.Trim(line, "[]")
			// Accept both `[colors.primary]` and `[primary]`.
			s = strings.TrimPrefix(s, "colors.")
			current = s
			if _, ok := sections[current]; !ok {
				sections[current] = map[string]string{}
			}
			continue
		}
		key, val, ok := splitTOMLKV(line)
		if !ok || current == "" {
			continue
		}
		sections[current][key] = val
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	prim := sections["primary"]
	norm := sections["normal"]
	brig := sections["bright"]
	curs := sections["cursor"]

	a := &Alacritty{
		BG: pickHex(prim, "background", "#000000"),
		FG: pickHex(prim, "foreground", "#FFFFFF"),
	}
	a.Cursor = pickHex(curs, "cursor", a.FG)
	a.CursorText = pickHex(curs, "text", a.BG)

	normSlots := [8]*string{&a.Black, &a.Red, &a.Green, &a.Yellow, &a.Blue, &a.Magenta, &a.Cyan, &a.White}
	brigSlots := [8]*string{&a.BBlack, &a.BRed, &a.BGreen, &a.BYellow, &a.BBlue, &a.BMagenta, &a.BCyan, &a.BWhite}
	shortNames := [8]string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white"}

	for i, name := range shortNames {
		*normSlots[i] = pickHex(norm, name, ansiDefaults[i])
		// Bright falls back to the corresponding normal color when missing
		// (Python theme-derive parity: `out[f"ansi{i+8}"] = _norm(bright.get(name, out[f"ansi{i}"]))`).
		*brigSlots[i] = pickHex(brig, name, *normSlots[i])
	}

	return a, nil
}

// pickHex returns section[key] normalized as hex, or def normalized.
func pickHex(section map[string]string, key, def string) string {
	if v, ok := section[key]; ok && v != "" {
		return normHex(v)
	}
	return normHex(def)
}

// get returns a color by named key ("bg", "fg", "cursor", "cursor_text",
// "green", "b_green", "ansi0"..."ansi15"). ok=false for unknown names.
//
// Used by Palette.Role/Var as the alacritty-fallback layer.
func (a *Alacritty) get(name string) (string, bool) {
	switch name {
	case "bg":
		return a.BG, true
	case "fg":
		return a.FG, true
	case "cursor":
		return a.Cursor, true
	case "cursor_text":
		return a.CursorText, true
	}
	// ansi0..ansi15
	if strings.HasPrefix(name, "ansi") {
		var idx int
		if _, err := fmt.Sscanf(name, "ansi%d", &idx); err == nil && idx >= 0 && idx < 16 {
			return a.AnsiN(idx), true
		}
	}
	// short names.
	switch name {
	case "black":
		return a.Black, true
	case "red":
		return a.Red, true
	case "green":
		return a.Green, true
	case "yellow":
		return a.Yellow, true
	case "blue":
		return a.Blue, true
	case "magenta":
		return a.Magenta, true
	case "cyan":
		return a.Cyan, true
	case "white":
		return a.White, true
	case "b_black":
		return a.BBlack, true
	case "b_red":
		return a.BRed, true
	case "b_green":
		return a.BGreen, true
	case "b_yellow":
		return a.BYellow, true
	case "b_blue":
		return a.BBlue, true
	case "b_magenta":
		return a.BMagenta, true
	case "b_cyan":
		return a.BCyan, true
	case "b_white":
		return a.BWhite, true
	}
	return "", false
}

// AnsiN returns ANSI color slot 0..15. Out-of-range returns #000000.
func (a *Alacritty) AnsiN(i int) string {
	if i < 0 || i >= 16 {
		return "#000000"
	}
	all := [16]string{
		a.Black, a.Red, a.Green, a.Yellow, a.Blue, a.Magenta, a.Cyan, a.White,
		a.BBlack, a.BRed, a.BGreen, a.BYellow, a.BBlue, a.BMagenta, a.BCyan, a.BWhite,
	}
	return all[i]
}

// NamedANSI returns the alacritty-name (e.g. "b_green") for ANSI slot i,
// used by tests and error messages.
func NamedANSI(i int) string {
	if i < 0 || i >= 16 {
		return ""
	}
	return ansiNames[i]
}
