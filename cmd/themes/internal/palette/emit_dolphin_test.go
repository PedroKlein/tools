package palette_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

func TestDolphinColors(t *testing.T) {
	for _, appearance := range []string{"dark", "light"} {
		t.Run(appearance, func(t *testing.T) {
			theme, err := palette.Load(filepath.Join("testdata", "osaka-jade"))
			if err != nil {
				t.Fatal(err)
			}

			theme.Appearance = appearance
			theme.Palette.Semantic.Bg = "#102030"
			theme.Palette.Semantic.Fg = "#abcdef"
			theme.Palette.Semantic.Accent = "#123"
			theme.Palette.Semantic.SelectionBg = "#778899aa"

			var emitter palette.Emitter

			for _, e := range palette.EmittersV4 {
				if e.App() == "dolphin" {
					emitter = e
				}
			}

			if emitter == nil {
				t.Fatal("Dolphin emitter missing")
			}

			var out bytes.Buffer
			if err := emitter.Emit(theme, &out); err != nil {
				t.Fatal(err)
			}

			icons := "Papirus-Dark"
			if appearance == "light" {
				icons = "Papirus"
			}

			for _, want := range []string{"[Colors:View]\nBackgroundNormal=16,32,48", "ForegroundNormal=171,205,239", "DecorationFocus=17,34,51", "[Colors:Selection]\nBackgroundNormal=119,136,153", "Theme=" + icons, "widgetStyle=Breeze"} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q in %s", want, out.String())
				}
			}
		})
	}
}
