package palette

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmittersWave5MacosAgainstOsakaJade — macos.json emitter reads
// theme.Macos and produces the schema the macos-system.sh hook expects.
func TestEmittersWave5MacosAgainstOsakaJade(t *testing.T) {
	th, err := LoadTheme(filepath.Join("testdata", "osaka-jade"))
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	got := map[string]Emitter{}
	for _, e := range EmittersV4 {
		got[e.App()] = e
	}
	e := got["macos"]
	if e == nil {
		t.Fatal("macos emitter not registered")
	}

	var buf bytes.Buffer
	if err := e.Emit(th, &buf); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	// Valid JSON.
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("macos.json is not valid JSON: %v\n%s", err, buf.String())
	}
	// Osaka-jade explicitly sets macos.accent="green" (case-insensitive
	// match to preset "Green" -> int 3).
	if m["accent_preset"] != "Green" {
		t.Errorf("accent_preset = %v, want Green", m["accent_preset"])
	}
	if m["accent_int"].(float64) != 3 {
		t.Errorf("accent_int = %v, want 3", m["accent_int"])
	}
	if m["mode"] != "dark" {
		t.Errorf("mode = %v, want dark", m["mode"])
	}
}

// TestEmittersWave5MacosHueMatchFallback — with macos.accent omitted,
// the emitter hue-matches from palette.semantic.accent.
func TestEmittersWave5MacosHueMatchFallback(t *testing.T) {
	// osaka-jade with macos.accent stripped, keeping the raw green
	// accent color: expect hue-match to Green (or Graphite for
	// low-saturation, but osaka-jade's accent is saturated enough).
	raw := `{
		"name": "hue-test",
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
				"accent":"#549E6A","error":"#f55","warning":"#ec7","ok":"#5a8"
			}
		}
	}`
	th, err := decodeTheme(strings.NewReader(raw), "/tmp/hue-test")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var buf bytes.Buffer
	if err := (macosEmitter{}).Emit(th, &buf); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	if m["accent_preset"] != "Green" {
		t.Errorf("hue-match: accent_preset = %v, want Green", m["accent_preset"])
	}
}

// TestWallpaperResolveHonorsDefaultAndPlacement — AC-2: resolver honors
// wallpapers.default and wallpapers.placement.
func TestWallpaperResolveHonorsDefaultAndPlacement(t *testing.T) {
	dir := t.TempDir()
	bg := filepath.Join(dir, "backgrounds")
	if err := os.MkdirAll(bg, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create three fake image files.
	for _, name := range []string{"alpha.jpg", "beta.jpg", "gamma.jpg"} {
		if err := os.WriteFile(filepath.Join(bg, name), []byte("fake"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	th := &Theme{
		Dir: dir,
		Wallpapers: Wallpapers{
			Default:   "beta.jpg",
			Placement: "fit",
		},
	}
	path, placement, err := ResolveWallpaper(th, "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if filepath.Base(path) != "beta.jpg" {
		t.Errorf("want beta.jpg, got %s", filepath.Base(path))
	}
	if placement != "fit" {
		t.Errorf("placement = %q, want fit", placement)
	}
}

// TestWallpaperResolveOverrideWins — override takes precedence when the
// file exists.
func TestWallpaperResolveOverrideWins(t *testing.T) {
	dir := t.TempDir()
	bg := filepath.Join(dir, "backgrounds")
	os.MkdirAll(bg, 0o755)
	os.WriteFile(filepath.Join(bg, "one.jpg"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(bg, "two.jpg"), []byte("y"), 0o644)
	th := &Theme{Dir: dir, Wallpapers: Wallpapers{Default: "one.jpg"}}
	path, _, err := ResolveWallpaper(th, "two.jpg")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if filepath.Base(path) != "two.jpg" {
		t.Errorf("want two.jpg override, got %s", filepath.Base(path))
	}
}

// TestWallpaperResolveFallsBackToFirstImage — when neither default nor
// list resolves, alphabetically first image wins.
func TestWallpaperResolveFallsBackToFirstImage(t *testing.T) {
	dir := t.TempDir()
	bg := filepath.Join(dir, "backgrounds")
	os.MkdirAll(bg, 0o755)
	os.WriteFile(filepath.Join(bg, "zeta.jpg"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(bg, "alpha.jpg"), []byte("y"), 0o644)
	th := &Theme{Dir: dir}
	path, _, err := ResolveWallpaper(th, "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if filepath.Base(path) != "alpha.jpg" {
		t.Errorf("want alpha.jpg, got %s", filepath.Base(path))
	}
}
