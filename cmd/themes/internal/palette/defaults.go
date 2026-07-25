package palette

import (
	"fmt"
	"strings"
)

// This file holds the small helpers used by the default-fill logic in
// loader.go.

// trimFloat formats a float using Python-style repr: trailing zeros
// stripped, but at least one decimal for whole numbers.
// 0.85 -> "0.85", 1.0 -> "1.0", 0.5 -> "0.5".
func trimFloat(f float64) string {
	s := fmt.Sprintf("%g", f)
	if !strings.Contains(s, ".") && !strings.Contains(s, "e") {
		s += ".0"
	}
	return s
}

// darken returns `hex` with every RGB channel scaled by (1 - pct).
// pct is clamped to [0, 1]. Returns "#000000" on parse garbage (matches
// hexToRGB's fallback).
//
// Used by fillSemanticDefaults for bg_alt.
func darken(hex string, pct float64) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	r, g, b := hexToRGB(hex)
	scale := 1 - pct
	return fmt.Sprintf("#%02x%02x%02x",
		int(float64(r)*scale),
		int(float64(g)*scale),
		int(float64(b)*scale),
	)
}

// mix returns a linear interpolation between hex a and hex b at ratio t.
// t=0 returns a, t=1 returns b, t=0.5 returns their midpoint. t is
// clamped to [0, 1].
//
// Used by fillSemanticDefaults for fg_dim.
func mix(a, b string, t float64) string {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	ar, ag, ab := hexToRGB(a)
	br, bg, bb := hexToRGB(b)
	return fmt.Sprintf("#%02x%02x%02x",
		int(float64(ar)*(1-t)+float64(br)*t),
		int(float64(ag)*(1-t)+float64(bg)*t),
		int(float64(ab)*(1-t)+float64(bb)*t),
	)
}
