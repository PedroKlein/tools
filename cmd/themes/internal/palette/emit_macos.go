package palette

import (
	"fmt"
	"io"
	"strings"
)

// v4 macos emitter — writes macos.json for the Go macOS reload hook.
//
// Reads theme.Macos.{appearance,accent,highlight} which the loader has
// already default-filled from top-level appearance + palette.semantic.accent.
//
// Output is strict JSON (no comment markers).

type macosEmitter struct{}

func (macosEmitter) App() string      { return "macos" }
func (macosEmitter) Filename() string { return "macos.json" }

func (macosEmitter) Emit(t *Theme, w io.Writer) error {
	mode := t.Macos.Appearance
	if mode == "system" || mode == "" {
		if IsLight(t.Palette.Semantic.Bg) {
			mode = "light"
		} else {
			mode = "dark"
		}
	}

	accentHex := t.Palette.Semantic.Accent

	// Resolve preset: either an explicit name from theme.macos.accent,
	// or hue-match from the accent color.
	var preset MacPreset
	if p, ok := lookupPresetByName(t.Macos.Accent); ok {
		preset = p
	} else {
		preset = MatchMacPreset(accentHex)
	}

	// Highlight: explicit theme.macos.highlight, else the accent hex.
	hlHex := t.Macos.Highlight
	if hlHex == "" {
		hlHex = accentHex
	}
	r, g, b := hexToRGB(hlHex)
	highlight := fmt.Sprintf("%.3f %.3f %.3f Theme Accent",
		float64(r)/255, float64(g)/255, float64(b)/255)

	fmt.Fprintf(w, `{
	"mode": %q,
	"accent_hex": %q,
	"accent_preset": %q,
	"accent_int": %d,
	"highlight_rgb": %q
}
`, mode, accentHex, preset.Name, preset.Int, highlight)
	return nil
}

// lookupPresetByName finds a preset by case-insensitive name. Returns
// (zero, false) if `name` is empty or "auto" (caller falls back to
// hue-match).
func lookupPresetByName(name string) (MacPreset, bool) {
	if name == "" || strings.EqualFold(name, "auto") {
		return MacPreset{}, false
	}
	for _, p := range append(macPresets, Graphite) {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return MacPreset{}, false
}
