package palette

import (
	"fmt"
	"io"
	"strings"
)

type dolphinEmitter struct{}

func (dolphinEmitter) App() string      { return "dolphin" }
func (dolphinEmitter) Filename() string { return "dolphin.colors" }
func (e dolphinEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", func(t *Theme, w io.Writer) error {
		s := t.Palette.Semantic
		for _, group := range []string{"View", "Window", "Button", "Tooltip", "Complementary", "Header", "Selection"} {
			bg, fg := s.Bg, s.Fg
			if group == "Window" || group == "Button" || group == "Header" {
				bg = s.BgAlt
			}

			if group == "Selection" {
				bg, fg = s.SelectionBg, s.SelectionFg
			}

			_, err := fmt.Fprintf(w, "[Colors:%s]\nBackgroundNormal=%s\nBackgroundAlternate=%s\nForegroundNormal=%s\nForegroundInactive=%s\nForegroundActive=%s\nForegroundLink=%s\nForegroundVisited=%s\nForegroundNegative=%s\nForegroundNeutral=%s\nForegroundPositive=%s\nDecorationFocus=%s\nDecorationHover=%s\n\n", group, kdeRGB(bg), kdeRGB(s.BgAlt), kdeRGB(fg), kdeRGB(s.Muted), kdeRGB(s.Accent), kdeRGB(s.Info), kdeRGB(s.Accent2), kdeRGB(s.Error), kdeRGB(s.Warning), kdeRGB(s.Ok), kdeRGB(s.Accent), kdeRGB(s.Accent))
			if err != nil {
				return fmt.Errorf("emit Dolphin colors: %w", err)
			}
		}

		icons := "Papirus-Dark"
		if t.Appearance == "light" {
			icons = "Papirus"
		}

		_, err := fmt.Fprintf(w, "[Icons]\nTheme=%s\n\n[KDE]\nwidgetStyle=Breeze\n", icons)
		if err != nil {
			return fmt.Errorf("emit Dolphin appearance: %w", err)
		}

		return nil
	}, NoHints)
}

func kdeRGB(color string) string {
	hex := strings.TrimPrefix(color, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}

	if len(hex) == 8 {
		hex = hex[:6]
	}

	r, g, b := hexToRGB(hex)

	return fmt.Sprintf("%d,%d,%d", r, g, b)
}
