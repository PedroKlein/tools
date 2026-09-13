package palette

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmittersWave4SmokeAgainstOsakaJade runs k9s/television/btop/opencode
// against osaka-jade. Also verifies AC-2: btop emitter uses
// palette.gradients.* when present.
func TestEmittersWave4SmokeAgainstOsakaJade(t *testing.T) {
	th, err := Load(filepath.Join("testdata", "osaka-jade"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := map[string]Emitter{}
	for _, e := range EmittersV4 {
		got[e.App()] = e
	}

	t.Run("k9s", func(t *testing.T) {
		var buf bytes.Buffer
		if err := got["k9s"].Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		out := buf.String()
		for _, want := range []string{"k9s:", "body:", "frame:", "views:", "yaml:"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q:\n%s", want, out)
			}
		}
		if strings.Count(out, "\nk9s:") != 1 {
			t.Errorf("k9s skin must contain one top-level k9s key; got:\n%s", out)
		}
		for _, want := range []string{"cursorFgColor:", "cursorBgColor:"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("television", func(t *testing.T) {
		var buf bytes.Buffer
		if err := got["television"].Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}

		assertTelevisionTheme(t, buf.String())
	})

	t.Run("btop-with-gradients", func(t *testing.T) {
		var buf bytes.Buffer
		if err := got["btop"].Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		out := buf.String()
		// osaka-jade sets explicit gradients; btop output must use them.
		want := `theme[temp_start]="` + th.Palette.Gradients.Temp[0] + `"`
		if !strings.Contains(out, want) {
			t.Errorf("btop gradient temp_start not from theme; want %q\n%s", want, out)
		}
	})

	t.Run("btop-without-gradients", func(t *testing.T) {
		// Build a minimal theme, let defaults derive gradients.
		minTh, err := decodeTheme(strings.NewReader(minimalThemeJSON), "/tmp/min")
		if err != nil {
			t.Fatalf("decode minimal: %v", err)
		}
		var buf bytes.Buffer
		if err := got["btop"].Emit(minTh, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		out := buf.String()
		// Derived gradients start with Warning for temp[0].
		want := `theme[temp_start]="` + minTh.Palette.Gradients.Temp[0] + `"`
		if !strings.Contains(out, want) {
			t.Errorf("btop default-derived gradient missing: want %q\n%s", want, out)
		}
	})

	t.Run("opencode", func(t *testing.T) {
		var buf bytes.Buffer
		if err := got["opencode"].Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		if strings.TrimSpace(buf.String()) != "osaka-jade" {
			t.Errorf("opencode output = %q, want osaka-jade", strings.TrimSpace(buf.String()))
		}
	})

	t.Run("tuicr", func(t *testing.T) {
		var buf bytes.Buffer
		if err := got["tuicr"].Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		out := buf.String()
		for _, want := range []string{"panel_bg =", "diff_add =", "syntax_theme = \"current.tmTheme\"", "mode_bg ="} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("obsidian", func(t *testing.T) {
		var buf bytes.Buffer
		if err := got["obsidian"].Emit(th, &buf); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		out := buf.String()
		for _, want := range []string{"--background-primary", "--text-normal", "--interactive-accent", "--code-keyword"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q:\n%s", want, out)
			}
		}
	})
}

func assertTelevisionTheme(t *testing.T, out string) {
	t.Helper()

	wantFields := []string{
		`background = "#111C18"`,
		`border_fg = "#509475"`,
		`text_fg = "#C1C497"`,
		`dimmed_text_fg = "#627A6C"`,
		`input_text_fg = "#549E6A"`,
		`result_count_fg = "#627A6C"`,
		`result_name_fg = "#E5C736"`,
		`result_line_number_fg = "#E5C736"`,
		`result_value_fg = "#C1C497"`,
		`selection_fg = "#C1C497"`,
		`selection_bg = "#23372B"`,
		`match_fg = "#E5C736"`,
		`preview_title_fg = "#549E6A"`,
		`channel_mode_fg = "#111C18"`,
		`channel_mode_bg = "#E5C736"`,
		`remote_control_mode_fg = "#111C18"`,
		`remote_control_mode_bg = "#63B07A"`,
		`action_picker_mode_fg = "#111C18"`,
		`action_picker_mode_bg = "#549E6A"`,
	}

	for _, want := range wantFields {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}

	if strings.Contains(out, "[") {
		t.Errorf("standalone theme contains a TOML table:\n%s", out)
	}
}

// minimalThemeJSON is a valid theme.json with only required fields set.
const minimalThemeJSON = `{
	"name": "min",
	"appearance": "dark",
	"palette": {
		"ansi": [
			"#000000","#800000","#008000","#808000",
			"#000080","#800080","#008080","#c0c0c0",
			"#808080","#ff0000","#00ff00","#ffff00",
			"#0000ff","#ff00ff","#00ffff","#ffffff"
		],
		"semantic": {
			"bg":"#111","fg":"#eee","muted":"#888",
			"accent":"#7a8","error":"#f55","warning":"#ec7","ok":"#5a8"
		}
	}
}`
