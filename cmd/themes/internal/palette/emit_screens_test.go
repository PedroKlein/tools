package palette_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

func TestHyprlockIncludesSharedFontFamily(t *testing.T) {
	theme, err := palette.Load(filepath.Join("testdata", "osaka-jade"))
	if err != nil {
		t.Fatal(err)
	}

	out := emitScreen(t, theme, "hyprlock")
	if !strings.Contains(string(out), "$screen_font = "+theme.Typography.Family+"\n") {
		t.Fatal("lock palette lost shared typography")
	}
}

func TestScreenEmitters(t *testing.T) {
	for _, appearance := range []string{"dark", "light"} {
		t.Run(appearance, func(t *testing.T) {
			theme, err := palette.Load(filepath.Join("testdata", "osaka-jade"))
			if err != nil {
				t.Fatal(err)
			}

			theme.Appearance = appearance
			if appearance == "light" {
				theme.Palette.Semantic.Bg = "#eeeeee"
				theme.Palette.Semantic.Fg = "#111111"
			}

			for _, app := range []string{"hyprlock", "sddm"} {
				t.Run(app, func(t *testing.T) {
					out := emitScreen(t, theme, app)
					if app == "hyprlock" {
						checkLockColors(t, theme, out)
					} else {
						checkLoginColors(t, theme, out)
					}
				})
			}
		})
	}
}

func emitScreen(t *testing.T, theme *palette.Theme, app string) []byte {
	t.Helper()

	for _, emitter := range palette.EmittersV4 {
		if emitter.App() != app {
			continue
		}

		theme.Overrides = map[string]string{app: "arbitrary override"}

		var out bytes.Buffer
		if err := emitter.Emit(theme, &out); err != nil {
			t.Fatal(err)
		}

		return out.Bytes()
	}

	t.Fatalf("%s emitter missing", app)

	return nil
}

func checkLockColors(t *testing.T, theme *palette.Theme, out []byte) {
	t.Helper()

	for _, want := range []string{
		"$screen_bg = rgb(" + strings.TrimPrefix(theme.Palette.Semantic.Bg, "#") + ")",
		"$screen_fg = rgb(" + strings.TrimPrefix(theme.Palette.Semantic.Fg, "#") + ")",
		"arbitrary override",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}

func checkLoginColors(t *testing.T, theme *palette.Theme, out []byte) {
	t.Helper()

	var values map[string]string
	if err := json.Unmarshal(out, &values); err != nil {
		t.Fatalf("sddm output must remain data-only JSON: %v", err)
	}

	if values["background"] != theme.Palette.Semantic.Bg || values["foreground"] != theme.Palette.Semantic.Fg || values["accent"] != theme.Palette.Semantic.Accent || values["font"] != theme.Typography.Family {
		t.Fatalf("sddm values do not follow theme: %#v", values)
	}
}
