package palette

import (
	"fmt"
	"io"
	"strings"
)

// EmitMacOS writes a `.macos.json` sidecar with precomputed macOS
// integration values so the runtime hook (macos-system.sh) can apply the
// theme with a single `defaults write` chain — no hue math at hook time.
//
// Schema:
//
//	{
//	  "mode": "dark" | "light",
//	  "accent_hex": "#549E6A",
//	  "accent_preset": "Green",
//	  "accent_int": 3,
//	  "highlight_rgb": "0.329 0.620 0.416 Theme Accent"
//	}
//
// mode is either `[meta] mode` from palette.toml OR auto-detected via YIQ
// on the alacritty background (light if YIQ > 128).
//
// highlight_rgb is macOS's AppleHighlightColor format: three floats in
// [0,1] separated by spaces followed by the label "Theme Accent". macOS
// uses this exact string when the user picks "Accent Color" in Settings.
func EmitMacOS(w io.Writer, p *Palette) error {
	mode := p.MetaString("mode", "")
	if mode == "" {
		if IsLight(p.Alacritty.BG) {
			mode = "light"
		} else {
			mode = "dark"
		}
	}

	// Accent hex: [meta] accent → [vars] accent → alacritty green.
	accent := p.MetaString("accent", "")
	if accent == "" {
		accent = p.Var("accent", "green")
	}
	accent = normHex(accent)

	// Preset match unless [meta] accent_preset explicitly overrides.
	preset := MatchMacPreset(accent)
	if override := p.MetaString("accent_preset", ""); override != "" {
		for _, candidate := range append(macPresets, Graphite) {
			if strings.EqualFold(candidate.Name, override) {
				preset = candidate
				break
			}
		}
	}

	r, g, b := hexToRGB(accent)
	// Highlight uses accent hex by default; [meta] highlight_hex overrides
	// so themes whose preset was force-matched to a different color
	// (rose-pine → Pink, tokyonight → Purple) can pick a highlight from
	// within their palette that reads well against the preset chrome.
	if rawOverride := p.MetaString("highlight_hex", ""); rawOverride != "" {
		r, g, b = hexToRGB(normHex(rawOverride))
	}
	highlight := fmt.Sprintf("%.3f %.3f %.3f Theme Accent",
		float64(r)/255, float64(g)/255, float64(b)/255)

	fmt.Fprintf(w, `{
	"mode": %q,
	"accent_hex": %q,
	"accent_preset": %q,
	"accent_int": %d,
	"highlight_rgb": %q
}
`, mode, accent, preset.Name, preset.Int, highlight)
	return nil
}
