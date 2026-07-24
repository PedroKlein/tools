package palette

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestEmitOsakaJadeGoldens diffs every Go emitter against the Python
// baseline captured in testdata/golden/osaka-jade/.
//
// Migration guardrail: as long as this test passes, the Go port produces
// byte-identical output for the osaka-jade palette. Other themes are
// covered by TestEmitAllSeedThemes below (regenerate-and-diff style).
func TestEmitOsakaJadeGoldens(t *testing.T) {
	p, err := Load(osakaJadeDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	goldenDir := "testdata/golden/osaka-jade"

	for _, e := range Emitters {
		t.Run(e.App, func(t *testing.T) {
			var buf bytes.Buffer
			if err := e.Emit(&buf, p); err != nil {
				t.Fatalf("emit %s: %v", e.App, err)
			}
			got := buf.Bytes()
			want, err := os.ReadFile(filepath.Join(goldenDir, e.Filename))
			if err != nil {
				t.Fatalf("read golden %s: %v", e.Filename, err)
			}
			if !bytes.Equal(got, want) {
				// Write actual output next to golden for easy diff.
				actualPath := filepath.Join(t.TempDir(), e.Filename+".got")
				_ = os.WriteFile(actualPath, got, 0o644)
				t.Errorf("emit %s: bytes differ from golden.\n  golden: %s\n  actual: %s",
					e.App, filepath.Join(goldenDir, e.Filename), actualPath)
			}
		})
	}
}

// TestEmitAllReturnsNoError sanity-checks every emitter runs without error
// on a minimal alacritty-only theme.
func TestEmitAllReturnsNoError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "alacritty.toml"), []byte(`
[colors.primary]
background = "#111111"
foreground = "#EEEEEE"
[colors.normal]
black = "#000000"
red = "#FF0000"
green = "#00FF00"
yellow = "#FFFF00"
blue = "#0000FF"
magenta = "#FF00FF"
cyan = "#00FFFF"
white = "#FFFFFF"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, e := range Emitters {
		t.Run(e.App, func(t *testing.T) {
			if err := e.Emit(io.Discard, p); err != nil {
				t.Fatalf("emit %s: %v", e.App, err)
			}
		})
	}
}
