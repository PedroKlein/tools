package palette_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

func TestArchEmitters(t *testing.T) {
	theme, err := palette.Load(filepath.Join("testdata", "osaka-jade"))
	if err != nil {
		t.Fatal(err)
	}

	for _, app := range []string{"hyprland", "waybar"} {
		t.Run(app, func(t *testing.T) {
			var emitter palette.Emitter

			for _, e := range palette.EmittersV4 {
				if e.App() == app {
					emitter = e
				}
			}

			if emitter == nil {
				t.Fatalf("%s emitter missing", app)
			}

			theme.Overrides = map[string]string{app: "\ncustom override\n"}
			if app == "hyprland" {
				theme.Palette.Semantic.Accent = "#abc"
				theme.Palette.Semantic.Border = "#11223344"
			}

			var out bytes.Buffer
			if err := emitter.Emit(theme, &out); err != nil {
				t.Fatal(err)
			}

			s := theme.Palette.Semantic

			expected := []string{"@define-color bg " + s.Bg + ";", "@define-color fg " + s.Fg + ";", "@define-color accent " + s.Accent + ";", "@define-color muted " + s.Muted + ";", "@define-color error " + s.Error + ";", "@define-color warning " + s.Warning + ";"}
			if app == "hyprland" {
				expected = []string{"hl.config({", "active_border = \"rgb(aabbcc)\"", "inactive_border = \"rgba(11223344)\""}
			}

			expected = append(expected, "custom override")

			for _, want := range expected {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q in %s", want, out.String())
				}
			}
		})
	}
}
