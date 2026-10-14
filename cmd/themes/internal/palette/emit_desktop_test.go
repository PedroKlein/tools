package palette_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

func TestDesktopColors(t *testing.T) {
	for _, appearance := range []string{"dark", "light"} {
		t.Run(appearance, func(t *testing.T) {
			theme, err := palette.Load(filepath.Join("testdata", "osaka-jade"))
			if err != nil {
				t.Fatal(err)
			}

			if appearance == "light" {
				theme.Palette.Semantic.Bg = "#eeeeee"
				theme.Palette.Semantic.Fg = "#111111"
			}

			for _, app := range []string{"fuzzel", "mako", "swayosd"} {
				t.Run(app, func(t *testing.T) { checkDesktopColors(t, theme, app) })
			}
		})
	}
}

func checkDesktopColors(t *testing.T, theme *palette.Theme, app string) {
	t.Helper()

	out := string(emitScreen(t, theme, app))
	s := theme.Palette.Semantic

	expected := []string{"background-color=" + s.Bg + "ee", "text-color=" + s.Fg, "border-color=" + s.Accent, "[urgency=critical]\nborder-color=" + s.Error}
	if app == "fuzzel" {
		expected = []string{"[colors]", "background=" + strings.TrimPrefix(s.Bg, "#") + "ee", "text=" + strings.TrimPrefix(s.Fg, "#") + "ff", "selection=" + strings.TrimPrefix(s.BgAlt, "#") + "ff", "selection-text=" + strings.TrimPrefix(s.Fg, "#") + "ff", "match=" + strings.TrimPrefix(s.Accent, "#") + "ff"}
	}

	if app == "swayosd" {
		expected = []string{"window#osd { background: " + s.Bg + ";", "border-color: " + s.Accent + ";", "color: " + s.Fg + ";", "background: " + s.Accent + ";"}
	}

	for _, want := range expected {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}

	for _, forbidden := range []string{"font=", "default-timeout=", "max-visible=", "width=", "lines="} {
		if strings.Contains(out, forbidden) {
			t.Errorf("color fragment changed layout/behavior: %s", out)
		}
	}
}
