package palette

import (
	"os"
	"path/filepath"
	"testing"
)

// osakaJadeDir returns the path to the dotfiles-tracked osaka-jade theme,
// which is our stable golden fixture for parser tests.
//
// Uses the checked-in dotfiles source rather than $HOME/.config/themes so
// tests are deterministic across machines and don't depend on `stow` being
// applied.
func osakaJadeDir(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	dir := filepath.Join(home, "dotfiles", "configs-shared", ".config", "themes", "osaka-jade")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	return dir
}

func TestPaletteLoad(t *testing.T) {
	p, err := Load(osakaJadeDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// AC: Vars["accent"] == "#549E6A" (osaka-jade fixed reference).
	if got := p.Vars["accent"]; got != "#549E6A" {
		t.Errorf("Vars[accent] = %q, want %q", got, "#549E6A")
	}
	// AC: Resolve("accent") returns the same via Role().
	if got := p.Role("accent", "green"); got != "#549E6A" {
		t.Errorf("Role(accent) = %q, want %q", got, "#549E6A")
	}
	// AC: Alacritty is populated.
	if p.Alacritty.BG == "" {
		t.Error("Alacritty.BG is empty; alacritty.toml not parsed")
	}
}

func TestRoleResolvesBothVarRefAndHex(t *testing.T) {
	p, err := Load(osakaJadeDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Case 1: role value is a var name → var lookup.
	// osaka-jade has: [roles] borderAccent = "bright"; [vars] bright = "#63B07A"
	if got := p.Role("borderAccent", "b_green"); got != "#63B07A" {
		t.Errorf("Role(borderAccent) via var-ref = %q, want %q", got, "#63B07A")
	}

	// Case 2: role value is a raw #hex → returned as-is.
	// osaka-jade has: [roles] customMessageBg = "#1A2024"
	if got := p.Role("customMessageBg", "b_black"); got != "#1A2024" {
		t.Errorf("Role(customMessageBg) via raw hex = %q, want %q", got, "#1A2024")
	}
}

func TestMetaParsing(t *testing.T) {
	// Synthetic palette.toml with [meta] block. Uses a temp dir so we
	// don't depend on any real theme having Meta set.
	dir := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(dir, "alacritty.toml"), []byte(`
[colors.primary]
background = "#111C18"
foreground = "#C1C497"
[colors.normal]
green = "#549E6A"
`), 0o644))
	must(os.WriteFile(filepath.Join(dir, "palette.toml"), []byte(`
[vars]
accent = "#549E6A"

[roles]
accent = "accent"

[meta]
mode = "dark"
opacity = 0.85
blur = 20
`), 0o644))

	p, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := p.MetaString("mode", ""); got != "dark" {
		t.Errorf("Meta.Mode = %q, want %q", got, "dark")
	}
	if got := p.MetaFloat("opacity", 0); got != 0.85 {
		t.Errorf("Meta.Opacity = %v, want %v", got, 0.85)
	}
	if got := p.MetaInt("blur", 0); got != 20 {
		t.Errorf("Meta.Blur = %d, want %d", got, 20)
	}
	// Absent key → default.
	if got := p.MetaFloat("nonexistent", 1.5); got != 1.5 {
		t.Errorf("MetaFloat default = %v, want %v", got, 1.5)
	}
}

func TestLoadWithoutPaletteTOML(t *testing.T) {
	// Theme dir with only alacritty.toml — Vars/Roles/Meta should be
	// empty but non-nil, and Alacritty should still populate.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "alacritty.toml"), []byte(`
[colors.primary]
background = "#000000"
foreground = "#FFFFFF"
[colors.normal]
green = "#00FF00"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(p.Vars) != 0 || len(p.Roles) != 0 || len(p.Meta) != 0 {
		t.Errorf("expected empty palette maps; got vars=%d roles=%d meta=%d",
			len(p.Vars), len(p.Roles), len(p.Meta))
	}
	if p.Alacritty.Green != "#00FF00" {
		t.Errorf("Alacritty.Green = %q; alacritty.toml not parsed", p.Alacritty.Green)
	}
}

func TestAlacrittyANSIFallbacks(t *testing.T) {
	// alacritty.toml with missing bright.red — should fall back to
	// normal.red (matches Python theme-derive behavior).
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "alacritty.toml"), []byte(`
[colors.primary]
background = "#111111"
foreground = "#EEEEEE"
[colors.normal]
red = "#AA0000"
[colors.bright]
green = "#00FF00"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// bright.red missing → falls back to normal.red.
	if p.Alacritty.BRed != "#AA0000" {
		t.Errorf("BRed = %q, want fallback to normal.red %q", p.Alacritty.BRed, "#AA0000")
	}
	// bright.green set → uses that.
	if p.Alacritty.BGreen != "#00FF00" {
		t.Errorf("BGreen = %q, want %q", p.Alacritty.BGreen, "#00FF00")
	}
}

func TestYIQ(t *testing.T) {
	tests := []struct {
		hex     string
		wantLum float64
		wantLt  bool // want IsLight
	}{
		{"#000000", 0, false},
		{"#FFFFFF", 255, true},
		{"#FF0000", 76.245, false},
		{"#00FF00", 149.685, true},
		{"#0000FF", 29.07, false},
		{"#549E6A", 129.946, true}, // osaka-jade accent — just above the threshold
	}
	for _, tt := range tests {
		got := YIQBrightness(tt.hex)
		// Allow small float rounding.
		if diff := got - tt.wantLum; diff > 0.5 || diff < -0.5 {
			t.Errorf("YIQBrightness(%s) = %v, want %v", tt.hex, got, tt.wantLum)
		}
		if IsLight(tt.hex) != tt.wantLt {
			t.Errorf("IsLight(%s) = %v, want %v", tt.hex, IsLight(tt.hex), tt.wantLt)
		}
	}
	// YIQContrast picks black on light, white on dark.
	if YIQContrast("#FFFFFF") != "#000000" {
		t.Error("YIQContrast(#FFFFFF) should be #000000")
	}
	if YIQContrast("#000000") != "#ffffff" {
		t.Error("YIQContrast(#000000) should be #ffffff")
	}
}

func TestParserInlineCommentOnQuotedValue(t *testing.T) {
	// Regression: palette.toml lines like `accent = "#549e6a" # comment`
	// used to drop the value because the parser required the closing quote
	// to be at the end of the trimmed line. Now the quoted region is
	// consumed independently and trailing text (comment or otherwise) is
	// discarded.
	tests := []struct {
		in       string
		wantKey  string
		wantVal  string
	}{
		{`accent = "#549e6a"`, "accent", "#549e6a"},
		{`accent = "#549e6a" # comment`, "accent", "#549e6a"},
		{`accent = "#549e6a"   # spaced comment`, "accent", "#549e6a"},
		{`accent = value # comment`, "accent", "value"},
		{`accent = value`, "accent", "value"},
		{`# whole-line comment`, "", ""}, // parser skips these earlier; splitTOMLKV never called
	}
	for _, tt := range tests {
		k, v, ok := splitTOMLKV(tt.in)
		if tt.wantKey == "" {
			// Not applicable to splitTOMLKV directly; skip.
			continue
		}
		if !ok || k != tt.wantKey || v != tt.wantVal {
			t.Errorf("splitTOMLKV(%q) = (%q, %q, %v), want (%q, %q, true)",
				tt.in, k, v, ok, tt.wantKey, tt.wantVal)
		}
	}
}

func TestNormHex(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"#549e6a", "#549E6A"},
		{`"#549e6a"`, "#549E6A"},
		{"549e6a", "#549E6A"},
		{"  #549e6a  ", "#549E6A"},
	}
	for _, tt := range tests {
		if got := normHex(tt.in); got != tt.want {
			t.Errorf("normHex(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
