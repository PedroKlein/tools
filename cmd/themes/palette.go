package main

import (
	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

// PaletteColors is the minimal set of colors the TUI needs to paint
// itself. Read from a theme's theme.json at TUI mount + on preview.
type PaletteColors struct {
	Accent     string
	Muted      string
	Bright     string
	SelectedBg string
	Fg         string
}

// Fallback colors matching osaka-jade. Used when theme.json is missing
// or unreadable so the TUI always has *some* theme.
var defaultPaletteColors = PaletteColors{
	Accent:     "#549E6A",
	Muted:      "#627A6C",
	Bright:     "#63B07A",
	SelectedBg: "#23372B",
	Fg:         "#C1C497",
}

// LoadPaletteColors reads <themeDir>/theme.json and returns the resolved
// TUI colors. Falls back to defaults on any error.
//
// v3 read from palette.toml [vars]+[roles]; v4 reads from
// palette.semantic.* which the schema guarantees are populated.
func LoadPaletteColors(themeDir string) PaletteColors {
	th, err := palette.Load(themeDir)
	if err != nil {
		return defaultPaletteColors
	}
	s := th.Palette.Semantic
	return PaletteColors{
		Accent:     s.Accent,
		Muted:      s.Muted,
		Bright:     s.Accent2,
		SelectedBg: s.SelectionBg,
		Fg:         s.Fg,
	}
}
