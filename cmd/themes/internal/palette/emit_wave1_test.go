package palette

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmittersWave1SmokeAgainstOsakaJade runs the 5 wave-1 emitters
// against the osaka-jade reference and asserts each output is
// non-empty, contains the 4 block markers, and has an app-specific
// smoke-check.
func TestEmittersWave1SmokeAgainstOsakaJade(t *testing.T) {
	th, err := Load(filepath.Join("testdata", "osaka-jade"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// The 5 wave-1 emitters we expect to be registered.
	wave1 := map[string]func(string) []string{
		"ghostty": func(out string) []string {
			return []string{
				"background = " + th.Palette.Semantic.Bg,
				"foreground = " + th.Palette.Semantic.Fg,
				"palette = 0=" + th.Palette.Ansi[0],
				"palette = 15=" + th.Palette.Ansi[15],
			}
		},
		"tmux": func(out string) []string {
			return []string{
				"pane-active-border-style",
				"window-status-current-format",
			}
		},
		"sketchybar": func(out string) []string {
			// Regression (E3): sketchybarrc + plugins in
			// configs-shared/.config/sketchybar/ reference every one of
			// these variables. Any missing export makes sketchybar fall
			// back to macOS system tint on icon.color, which is what the
			// user reported as 'app icons don't change color'.
			return []string{
				"export BAR_BG=0x",
				"export BAR_BORDER=0x",
				"export FG=0x",
				"export FG_MUTED=0x",
				"export FG_BRIGHT=0x",
				"export ACCENT=0x",
				"export ACCENT_BRIGHT=0x",
				"export ACCENT_ON=0x",
				"export HIGHLIGHT=0x",
				"export RED=0x",
				"export YELLOW=0x",
				"export GREEN=0x",
				"export CYAN=0x",
				"export MAGENTA=0x",
				"export INFO=0x",
				"export TEAL=0x",
				"export JADE=0x",
				"export VOLUME=0x",
				"export PERCENTAGE=0x",
				"export SURFACE=0x",
				"export SURFACE_LIGHT=0x",
				"export ICON=0x",
				"export CHARGING=0x",
				"export FOCUSED=0x",
				"export FOCUSED_WORKSPACE=0x",
				"export NON_EMPTY=0x",
				"export BADGE=0x",
			}
		},
		"aerospace": func(out string) []string {
			return []string{`focused_border = "` + th.Palette.Semantic.Accent + `"`}
		},
		"starship": func(out string) []string {
			return []string{`palette = "osaka-jade"`, "[palettes.osaka-jade]"}
		},
	}

	// Build an index of registered emitters by app name.
	got := map[string]Emitter{}
	for _, e := range EmittersV4 {
		got[e.App()] = e
	}

	for app, expectations := range wave1 {
		t.Run(app, func(t *testing.T) {
			e, ok := got[app]
			if !ok {
				t.Fatalf("emitter %q not registered in EmittersV4", app)
			}
			var buf bytes.Buffer
			if err := e.Emit(th, &buf); err != nil {
				t.Fatalf("Emit(%s): %v", app, err)
			}
			out := buf.String()
			if !strings.Contains(out, "--- baseline ---") ||
				!strings.Contains(out, "--- semantic ---") ||
				!strings.Contains(out, "--- hints ---") {
				t.Errorf("%s output missing block markers:\n%s", app, out)
			}
			for _, want := range expectations(out) {
				if !strings.Contains(out, want) {
					t.Errorf("%s output missing %q; got:\n%s", app, want, out)
				}
			}
		})
	}
}
