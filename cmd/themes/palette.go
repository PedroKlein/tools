package main

import (
	"os"
	"path/filepath"
	"strings"
)

// PaletteColors is the minimal set of colors the TUI needs to paint
// itself. Read from a theme's palette.toml at TUI mount + on preview.
type PaletteColors struct {
	// Border / heading accent.
	Accent string
	// Muted foreground for subtitles, help text, unselected items.
	Muted string
	// Bright foreground for selected items.
	Bright string
	// Selected row background.
	SelectedBg string
	// Body foreground.
	Fg string
}

// Fallback colors matching osaka-jade. Used when palette.toml is
// missing or unreadable so the TUI always has *some* theme.
var defaultPaletteColors = PaletteColors{
	Accent:     "#549E6A",
	Muted:      "#627A6C",
	Bright:     "#63B07A",
	SelectedBg: "#23372B",
	Fg:         "#C1C497",
}

// LoadPaletteColors reads <themeDir>/palette.toml and returns the
// resolved TUI colors. Falls back to defaults for any missing keys.
//
// palette.toml layout (from theme-derive):
//
//	[vars]
//	accent = "#549E6A"
//	muted  = "#627A6C"
//	bright = "#63B07A"
//	surface1 = "#23372B"
//	fg = "#C1C497"
//	[roles]
//	selectedBg = "surface1"     # role references a var by name
//	borderAccent = "bright"
//
// Roles may reference vars by name; we resolve them here.
func LoadPaletteColors(themeDir string) PaletteColors {
	p := defaultPaletteColors
	path := filepath.Join(themeDir, "palette.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return p
	}

	vars, roles := parsePaletteToml(string(data))

	// resolve resolves a role's value: either a raw #hex or a var lookup.
	resolve := func(v string) string {
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, "#") {
			return v
		}
		if hex, ok := vars[v]; ok {
			return hex
		}
		return ""
	}

	// Prefer role over var when both exist. Roles can express intent
	// ("selectedBg" -> "surface1") that a plain var lookup cannot.
	pick := func(role, varName string) string {
		if r, ok := roles[role]; ok {
			if hex := resolve(r); hex != "" {
				return hex
			}
		}
		if hex, ok := vars[varName]; ok {
			return hex
		}
		return ""
	}

	if v := pick("accent", "accent"); v != "" {
		p.Accent = v
	}
	if v := pick("muted", "muted"); v != "" {
		p.Muted = v
	}
	if v := vars["bright"]; v != "" {
		p.Bright = v
	}
	if v := pick("selectedBg", "surface1"); v != "" {
		p.SelectedBg = v
	}
	if v := vars["fg"]; v != "" {
		p.Fg = v
	}
	return p
}

// parsePaletteToml parses a very simple TOML subset for palette.toml:
//   - `[vars]` section: key = "#hex" or key = "value"
//   - `[roles]` section: key = "value"
//
// Ignores comments (# outside quotes) and anything not in a section.
// Sufficient for palette.toml which theme-derive emits deterministically.
func parsePaletteToml(s string) (vars, roles map[string]string) {
	vars = map[string]string{}
	roles = map[string]string{}
	section := ""
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		// Strip trailing comment.
		if hash := strings.Index(val, "#"); hash >= 0 && !strings.HasPrefix(val, "\"#") && !strings.HasPrefix(val, "'#") {
			// don't strip if the whole value is a #hex color
			if !(hash == 0 && len(val) >= 4 && isHexLiteral(val)) {
				// keep quoted #hex literals; only strip when # starts a comment
				// The simplest correct rule: strip # only if it's whitespace-prefixed.
				stripped := val[:hash]
				if strings.TrimSpace(stripped) != "" && strings.HasSuffix(strings.TrimRight(stripped, " \t"), " ") {
					val = strings.TrimSpace(stripped)
				}
			}
		}
		val = strings.Trim(val, `"'`)
		if val == "" {
			continue
		}
		switch section {
		case "vars":
			vars[key] = val
		case "roles":
			roles[key] = val
		}
	}
	return vars, roles
}

// isHexLiteral reports whether s starts with #RRGGBB (possibly quoted).
func isHexLiteral(s string) bool {
	s = strings.Trim(s, `"'`)
	if !strings.HasPrefix(s, "#") {
		return false
	}
	s = s[1:]
	if len(s) != 6 && len(s) != 3 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
