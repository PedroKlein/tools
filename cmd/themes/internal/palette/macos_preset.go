package palette

import "math"

// MacPreset is one of macOS's discrete accent color presets.
type MacPreset struct {
	// Name is the human-readable label from System Settings.
	Name string
	// Int is the value written to defaults `AppleAccentColor`.
	//   Multicolor: -2 (default across accent-aware apps; rarely used here)
	//   Graphite:   -1
	//   Red:         0
	//   Orange:      1
	//   Yellow:      2
	//   Green:       3
	//   Blue:        4
	//   Purple:      5
	//   Pink:        6
	Int int
	// Hue is the reference hue on the color wheel (degrees, 0..360).
	// Only meaningful for saturated presets; Graphite has no hue.
	Hue float64
}

// macPresets are the 7 colored + 1 Graphite presets, ordered so hue-nearest
// scans are stable. Graphite is handled separately by the saturation guard.
//
// Reference hues are eyeball-tuned to macOS's rendered accent swatches; a
// pure-color HSV wheel would put Green at 120° and Blue at 240°, but the
// System Settings Green swatch reads slightly more toward yellow-green
// and the Blue swatch is closer to true blue. Adjust if a specific theme
// misclassifies.
var macPresets = []MacPreset{
	{"Red", 0, 0},
	{"Orange", 1, 30},
	{"Yellow", 2, 55},
	{"Green", 3, 130},
	{"Blue", 4, 220},
	{"Purple", 5, 285},
	{"Pink", 6, 330},
}

// Graphite is the desaturated preset. Selected when the accent's HSV
// saturation is below graphiteSaturation.
var Graphite = MacPreset{Name: "Graphite", Int: -1, Hue: 0}

// graphiteSaturation is the cutoff below which a color is considered
// desaturated enough to map to Graphite. Empirically:
//   - #7DAEA3 (gruvbox-material teal): S ≈ 0.28  → colored preset
//   - #8B8B8B (true gray):              S = 0.00  → Graphite
//   - #C1C497 (osaka-jade fg cream):    S ≈ 0.23  → colored preset
//
// 0.15 is loose enough to keep the seven colored themes colored, tight
// enough to route true grays to Graphite.
const graphiteSaturation = 0.15

// MatchMacPreset returns the macOS accent preset closest to hex.
//
// Algorithm (documented in the plan doc):
//  1. Compute HSV of hex.
//  2. If saturation < graphiteSaturation → Graphite (no colored preset
//     would ever look correct for a near-gray accent).
//  3. Otherwise pick the preset with the smallest hue distance (on the
//     circular color wheel — 359° and 1° are 2° apart).
//
// Ignores value/brightness on purpose: users pick "Green" for both a
// forest green and a mint accent, and macOS renders both from the same
// preset int. Hue is what matters.
func MatchMacPreset(hex string) MacPreset {
	r, g, b := hexToRGB(hex)
	h, s, _ := rgbToHSV(float64(r)/255, float64(g)/255, float64(b)/255)

	if s < graphiteSaturation {
		return Graphite
	}

	best := macPresets[0]
	bestDist := hueDistance(h, best.Hue)
	for _, preset := range macPresets[1:] {
		d := hueDistance(h, preset.Hue)
		if d < bestDist {
			bestDist = d
			best = preset
		}
	}
	return best
}

// rgbToHSV converts sRGB in [0,1] to (H in degrees 0..360, S 0..1, V 0..1).
// Standard HSV formula — see e.g. https://en.wikipedia.org/wiki/HSL_and_HSV.
func rgbToHSV(r, gr, b float64) (h, s, v float64) {
	maxC := math.Max(r, math.Max(gr, b))
	minC := math.Min(r, math.Min(gr, b))
	v = maxC
	d := maxC - minC
	if maxC == 0 {
		return 0, 0, 0
	}
	s = d / maxC
	if d == 0 {
		return 0, s, v
	}
	switch maxC {
	case r:
		h = (gr - b) / d
		if gr < b {
			h += 6
		}
	case gr:
		h = (b-r)/d + 2
	case b:
		h = (r-gr)/d + 4
	}
	h *= 60
	return h, s, v
}

// hueDistance returns the shortest angular distance between two hues on
// the color wheel [0..360). Wraps around, so 359 and 1 are 2 apart.
func hueDistance(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > 180 {
		d = 360 - d
	}
	return d
}
