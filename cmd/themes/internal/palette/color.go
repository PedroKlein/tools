package palette

import "strings"

// hexToRGB parses `#RRGGBB` (case-insensitive) into (r, g, b) 0-255.
// Returns (0, 0, 0) on garbage — matches Python theme-derive fallback.
func hexToRGB(hex string) (r, g, b int) {
	s := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(s) != 6 {
		return 0, 0, 0
	}
	var parsed int
	for _, c := range s {
		parsed <<= 4
		switch {
		case c >= '0' && c <= '9':
			parsed |= int(c - '0')
		case c >= 'a' && c <= 'f':
			parsed |= int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			parsed |= int(c-'A') + 10
		default:
			return 0, 0, 0
		}
	}
	return (parsed >> 16) & 0xFF, (parsed >> 8) & 0xFF, parsed & 0xFF
}

// YIQBrightness returns the YIQ luma of hex, range 0..255.
//
//	luma = (299*R + 587*G + 114*B) / 1000
//
// Values above 128 read as "light"; below as "dark".
func YIQBrightness(hex string) float64 {
	r, g, b := hexToRGB(hex)
	return float64(r*299+g*587+b*114) / 1000.0
}

// YIQContrast returns "#000000" or "#ffffff" for maximum readability
// against the given color. Used by emit_tmux to choose the current-tab
// foreground so both light and dark themes stay legible.
//
// Note: lowercase "#ffffff" is intentional — matches the original Python
// theme-derive verbatim so migration goldens byte-diff clean.
func YIQContrast(hex string) string {
	if YIQBrightness(hex) > 128 {
		return "#000000"
	}
	return "#ffffff"
}

// IsLight reports whether hex reads as light (YIQ > 128).
// Used by emit_ghostty to pick the default translucency profile:
// higher opacity + lower blur on light themes (readable over bright wallpapers).
func IsLight(hex string) bool {
	return YIQBrightness(hex) > 128
}
