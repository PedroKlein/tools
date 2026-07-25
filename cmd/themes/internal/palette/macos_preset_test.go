package palette

import "testing"

func TestMacOSPresetMatch(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		// Expected preset. Wrap the acceptable set here so a legit
		// borderline classification (e.g. Green vs Blue for a teal)
		// doesn't fail the test.
		want    []string
		notWant []string
	}{
		// AC: gruvbox-material must not map to Graphite. #7DAEA3 is
		// muted teal (S ≈ 0.28) — routes to Green or Blue.
		{"gruvbox-material teal accent", "#7DAEA3", []string{"Green", "Blue"}, []string{"Graphite"}},

		// Osaka-jade forest green.
		{"osaka-jade accent", "#549E6A", []string{"Green"}, nil},

		// Catppuccin-latte's Blue accent.
		{"catppuccin-latte accent", "#1E66F5", []string{"Blue"}, nil},

		// Catppuccin-mocha's Blue accent.
		{"catppuccin-mocha accent", "#89B4FA", []string{"Blue"}, nil},

		// Tokyonight's Blue accent.
		{"tokyonight accent", "#7AA2F7", []string{"Blue"}, nil},

		// Rose-pine's teal/gold accent.
		// #9CCFD8 is desaturated teal — Green or Blue is fine.
		{"rose-pine accent", "#9CCFD8", []string{"Green", "Blue"}, []string{"Graphite"}},

		// Everforest's green-gold. #A7C080 is yellow-green (H≈83°), sitting
		// between Yellow (55°) and Green (130°) reference hues. Either is a
		// defensible mapping; if the user cares, they can pin via palette.toml
		// [meta] accent_preset override (planned future feature).
		{"everforest accent", "#A7C080", []string{"Green", "Yellow"}, []string{"Graphite"}},

		// True grays route to Graphite.
		{"middle gray", "#8B8B8B", []string{"Graphite"}, nil},
		{"near-black", "#222222", []string{"Graphite"}, nil},
		{"near-white", "#EEEEEE", []string{"Graphite"}, nil},

		// Saturated primaries route to their obvious preset.
		{"pure red", "#FF0000", []string{"Red"}, nil},
		{"pure orange", "#FF8000", []string{"Orange"}, nil},
		{"pure yellow", "#FFFF00", []string{"Yellow"}, nil},
		{"pure green", "#00FF00", []string{"Green"}, nil},
		{"pure blue", "#0080FF", []string{"Blue"}, nil},
		{"pure purple", "#8000FF", []string{"Purple"}, nil},
		{"pure pink", "#FF00A0", []string{"Pink"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchMacPreset(tt.hex)
			ok := false
			for _, w := range tt.want {
				if got.Name == w {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("MatchMacPreset(%s) = %s, want one of %v", tt.hex, got.Name, tt.want)
			}
			for _, banned := range tt.notWant {
				if got.Name == banned {
					t.Errorf("MatchMacPreset(%s) = %s, must NOT be %s", tt.hex, got.Name, banned)
				}
			}
		})
	}
}

func TestHueDistanceWraps(t *testing.T) {
	tests := []struct {
		a, b, want float64
	}{
		{0, 0, 0},
		{10, 20, 10},
		{350, 10, 20},  // wraps
		{359, 1, 2},    // wraps
		{180, 0, 180},  // exact half
		{200, 20, 180}, // slightly past half
		{300, 60, 120}, // symmetric
	}
	for _, tt := range tests {
		if got := hueDistance(tt.a, tt.b); got != tt.want {
			t.Errorf("hueDistance(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestRGBtoHSVBasics(t *testing.T) {
	// Sanity: black = (0, 0, 0); white = (any hue, 0, 1); pure red = (0, 1, 1).
	h, s, v := rgbToHSV(0, 0, 0)
	if s != 0 || v != 0 {
		t.Errorf("black: hsv=(%v,%v,%v), want (?,0,0)", h, s, v)
	}
	_, s, v = rgbToHSV(1, 1, 1)
	if s != 0 || v != 1 {
		t.Errorf("white: sat=%v v=%v, want (0,1)", s, v)
	}
	h, s, v = rgbToHSV(1, 0, 0)
	if h != 0 || s != 1 || v != 1 {
		t.Errorf("red: hsv=(%v,%v,%v), want (0,1,1)", h, s, v)
	}
}
