package palette

import "path/filepath"

// This file defines the theme.json v4 domain type. It coexists with the
// v3 Palette struct in palette.go until P1.11 deletes the v3 parser.
//
// Naming note: the P1.1 acceptance criterion asks for `Load(path string)
// (*Theme, error)`. Because the v3 code still exports `Load(themeDir
// string) (*Palette, error)`, the v4 loader is exported as `LoadTheme`
// for the duration of the port and will be renamed to `Load` in P1.11
// when the v3 parser is deleted.

// Theme is the fully-hydrated, default-filled representation of a v4
// theme.json file. Every field has a defined value after LoadTheme:
// optional fields carry their documented default.
//
// Emitters MUST read only from *Theme, not from raw theme.json bytes.
// This keeps the semantic contract centralized here.
type Theme struct {
	// Identity ------------------------------------------------------------

	// Name is the kebab-case identifier from theme.json.name. Also the
	// containing directory basename.
	Name string `json:"name"`

	// DisplayName is the optional human label. Falls back to Name when
	// theme.json omits it.
	DisplayName string `json:"displayName"`

	// Author is optional; empty string when omitted.
	Author string `json:"author"`

	// Upstream is the optional URL of the source repo (populated when the
	// theme was imported from a marketplace). Empty string when omitted.
	Upstream string `json:"upstream"`

	// Appearance is "dark" or "light". Required by schema.
	Appearance string `json:"appearance"`

	// Description is optional prose. Empty string when omitted.
	Description string `json:"description"`

	// Palette ------------------------------------------------------------

	Palette Colors `json:"palette"`

	// Optional sections --------------------------------------------------

	Typography Typography `json:"typography"`
	Effects    Effects    `json:"effects"`
	Macos      Macos      `json:"macos"`
	Wallpapers Wallpapers `json:"wallpapers"`

	// Hints are free-form structured overrides consumed by matching
	// emitters. Unknown keys are ignored, not rejected — this is the
	// LLM/schema-evolution escape hatch. Emitters look up
	// t.Hints[<app>].(map[string]any) and address well-known keys.
	Hints map[string]any `json:"hints"`

	// Overrides are raw fragments appended verbatim to derived files.
	// Keys ending in _path point to a sidecar file (relative to Dir)
	// whose contents are appended.
	Overrides map[string]string `json:"overrides"`

	// Runtime metadata ---------------------------------------------------

	// Dir is the absolute path to the theme directory containing
	// theme.json. Emitters resolve relative overrides paths against it.
	Dir string `json:"-"`

	// Unknown collects any top-level keys the schema did not know about,
	// so downstream code (e.g. TUI settings pane write-back) can round-
	// trip them. Populated by LoadTheme; not part of the schema itself.
	Unknown map[string]any `json:"-"`
}

// Colors is the top-level `palette` block from theme.json. It holds the
// ANSI 16, the semantic slots, and optional btop gradients.
//
// (Named Colors, not Palette, because the package already has a v3
// Palette type. v3 Palette will be deleted in P1.11 at which point this
// could be renamed to Palette; keeping Colors is fine too.)
type Colors struct {
	// Ansi is the 16-color ANSI palette in fixed order:
	// [black, red, green, yellow, blue, magenta, cyan, white,
	//  brBlack, brRed, brGreen, brYellow, brBlue, brMagenta, brCyan, brWhite].
	// Always length 16 after LoadTheme.
	Ansi [16]string `json:"ansi"`

	Semantic  Semantic  `json:"semantic"`
	Gradients Gradients `json:"gradients"`
}

// Semantic holds the semantic color slots. Required slots are always
// populated (schema enforces). Optional slots carry their documented
// default when theme.json omits them.
type Semantic struct {
	// Required slots.
	Bg      string `json:"bg"`
	Fg      string `json:"fg"`
	Muted   string `json:"muted"`
	Accent  string `json:"accent"`
	Error   string `json:"error"`
	Warning string `json:"warning"`
	Ok      string `json:"ok"`

	// Optional slots with defaults.
	Accent2     string `json:"accent2"`      // defaults to Accent
	Info        string `json:"info"`         // defaults to Accent
	SelectionBg string `json:"selection_bg"` // defaults to Accent
	SelectionFg string `json:"selection_fg"` // defaults to Fg
	Cursor      string `json:"cursor"`       // defaults to Accent2
	Border      string `json:"border"`       // defaults to Muted
	BgAlt       string `json:"bg_alt"`       // defaults to Bg darkened 5%
	FgDim       string `json:"fg_dim"`       // defaults to Fg mixed with Muted

	Git    GitColors    `json:"git"`
	Syntax SyntaxColors `json:"syntax"`
}

// GitColors are per-status colors. Defaults derive from Ok/Error/Warning.
type GitColors struct {
	Added    string `json:"added"`
	Removed  string `json:"removed"`
	Modified string `json:"modified"`
}

// SyntaxColors are language highlight scopes. Defaults derive from the
// ANSI palette (comment=brBlack, string=green, ...).
type SyntaxColors struct {
	Keyword  string `json:"keyword"`
	String   string `json:"string"`
	Number   string `json:"number"`
	Comment  string `json:"comment"`
	Type     string `json:"type"`
	Function string `json:"function"`
	Operator string `json:"operator"`
}

// Gradients are 3-color tuples consumed by btop.
type Gradients struct {
	Temp    [3]string `json:"temp"`
	Cpu     [3]string `json:"cpu"`
	Memory  [3]string `json:"memory"`
	Network [3]string `json:"network"`
}

// Typography is optional font settings.
type Typography struct {
	Family     string  `json:"family"`
	Size       float64 `json:"size"`
	Weight     string  `json:"weight"`
	Ligatures  bool    `json:"ligatures"`
	LineHeight float64 `json:"lineHeight"`
}

// Effects are visual chrome (opacity, blur, cursor, shadow).
type Effects struct {
	Opacity float64      `json:"opacity"`
	Blur    float64      `json:"blur"`
	Cursor  CursorEffect `json:"cursor"`
	Shadow  ShadowEffect `json:"shadow"`
}

type CursorEffect struct {
	Shape string `json:"shape"`
	Blink bool   `json:"blink"`
}

type ShadowEffect struct {
	Enabled    bool       `json:"enabled"`
	Radius     float64    `json:"radius"`
	OpacityPct float64    `json:"opacityPct"`
	Offset     [2]float64 `json:"offset"`
}

// Macos is macOS system integration (consumed by .hooks/macos-system.sh).
type Macos struct {
	// Appearance mirrors top-level Appearance unless the theme overrides it.
	Appearance string `json:"appearance"`

	// Accent is a preset name (one of macOS's 8 accent colors) or "auto"
	// for hue-match. Empty string means the loader hue-matched from
	// Palette.Semantic.Accent.
	Accent string `json:"accent"`

	// Highlight is the text-selection color. Defaults to
	// Palette.Semantic.Accent.
	Highlight string `json:"highlight"`
}

// Wallpapers describes the theme's wallpaper set.
type Wallpapers struct {
	// Default is the filename (in <Dir>/backgrounds/) picked when no
	// per-user preference exists.
	Default string `json:"default"`

	// Placement is fill|fit|center|stretch|tile. Defaults to fill.
	Placement string `json:"placement"`

	// List is every wallpaper the theme ships. Emitters may show a picker
	// over these files.
	List []Wallpaper `json:"list"`
}

type Wallpaper struct {
	File   string `json:"file"`
	Credit string `json:"credit"`
}

// Hint returns the map at t.Hints[app] as map[string]any, or nil if the
// key is missing / not an object. Emitters use this to fetch app-specific
// hint blocks defensively.
func (t *Theme) Hint(app string) map[string]any {
	if t == nil || t.Hints == nil {
		return nil
	}
	raw, ok := t.Hints[app]
	if !ok {
		return nil
	}
	m, _ := raw.(map[string]any)
	return m
}

// OverrideInline returns the verbatim string override for `app`, or "".
func (t *Theme) OverrideInline(app string) string {
	if t == nil || t.Overrides == nil {
		return ""
	}
	return t.Overrides[app]
}

// OverridePath returns the sidecar file path for `app` (resolved against
// t.Dir), or "" if unset.
func (t *Theme) OverridePath(app string) string {
	if t == nil || t.Overrides == nil {
		return ""
	}
	p, ok := t.Overrides[app+"_path"]
	if !ok || p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(t.Dir, p)
}
