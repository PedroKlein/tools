// Package palette parses theme colors and derives per-app config files.
//
// A theme lives on disk as a directory with:
//
//	alacritty.toml   (mandatory)  — 16 ANSI colors + primary + cursor
//	palette.toml     (optional)   — enriched semantic vars, roles, meta
//	.macos.json      (derived)    — precomputed macOS integration values
//	<13 app files>   (derived)    — ghostty.conf, tmux.conf, pi.json, ...
//
// The Palette type wraps both files and exposes:
//
//   - Alacritty: 16 ANSI colors + primary + cursor (upstream-authoritative)
//   - Vars:      raw hex colors from palette.toml [vars]
//   - Roles:     semantic role → var-name OR raw #hex, unresolved
//   - Meta:      free-form scalar bag (opacity, blur, mode overrides…)
//
// Callers use Role(name, fallback) to resolve a semantic role to a hex
// color; the fallback is an alacritty key ("green", "b_green", "bg"…).
//
// Design decisions:
//
//   - Bespoke TOML parser (~120 LoC). We only need [vars] [roles] [meta]
//     with `key = "value"` shape. Full TOML would need a 3kloc dep for
//     zero benefit.
//   - Roles preserve their ORIGINAL palette.toml value (var name or hex),
//     not the resolved hex. emit_pi.go needs the var name to emit
//     "accent = accent" style pi.json entries.
//   - Colors normalize to #UPPERCASE hex on ingest so equality checks and
//     golden files stay byte-stable across runs.
package palette

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Palette is the fully-parsed color state for one theme.
type Palette struct {
	// Alacritty is the ANSI + primary + cursor colors from alacritty.toml.
	// Always non-nil after Load().
	Alacritty *Alacritty

	// Vars are the raw named hex colors from palette.toml [vars].
	// Empty map if palette.toml is absent.
	Vars map[string]string

	// Roles map a semantic name to its unresolved palette.toml value.
	// A role value is either a var name (lookup in Vars) or a raw #hex.
	// Empty map if palette.toml is absent.
	Roles map[string]string

	// Meta is the free-form scalar bag from palette.toml [meta]. Values
	// come in as strings; use MetaFloat / MetaInt to parse.
	Meta map[string]string

	// Name is the theme directory basename (e.g. "osaka-jade"). Used by
	// emitters that need it (pi.json, starship, opencode, bat).
	Name string

	// Dir is the absolute path to the theme directory.
	Dir string
}

// Load reads a theme's alacritty.toml and palette.toml.
//
// alacritty.toml is mandatory — Load returns an error if missing or
// unparseable. palette.toml is optional — its absence just means empty
// Vars/Roles/Meta maps.
func Load(themeDir string) (*Palette, error) {
	abs, err := filepath.Abs(themeDir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", abs)
	}

	a, err := ParseAlacritty(filepath.Join(abs, "alacritty.toml"))
	if err != nil {
		return nil, err
	}

	vars, roles, meta, err := parsePaletteTOML(filepath.Join(abs, "palette.toml"))
	if err != nil {
		// palette.toml is optional — non-existence is fine; other errors
		// surface so authoring mistakes are visible.
		if !os.IsNotExist(err) {
			return nil, err
		}
		vars, roles, meta = map[string]string{}, map[string]string{}, map[string]string{}
	}

	return &Palette{
		Alacritty: a,
		Vars:      vars,
		Roles:     roles,
		Meta:      meta,
		Name:      filepath.Base(abs),
		Dir:       abs,
	}, nil
}

// Role resolves a semantic role to a hex color.
//
// Layered lookup:
//  1. If Roles[name] exists, resolve its value: raw #hex, or Vars[value].
//  2. If Vars[name] exists, return it.
//  3. If fallbackANSI names an Alacritty key (e.g. "green", "b_green"),
//     return that.
//  4. Otherwise normalize fallbackANSI as a hex color (#000000 on garbage).
func (p *Palette) Role(name, fallbackANSI string) string {
	if raw, ok := p.Roles[name]; ok {
		if strings.HasPrefix(raw, "#") {
			return normHex(raw)
		}
		if hex, ok := p.Vars[raw]; ok {
			return hex
		}
	}
	if hex, ok := p.Vars[name]; ok {
		return hex
	}
	if hex, ok := p.Alacritty.get(fallbackANSI); ok {
		return hex
	}
	return normHex(fallbackANSI)
}

// Var resolves a var name to a hex color, ignoring roles.
//
// Vars are the raw palette.toml [vars] entries; Var never traverses
// [roles]. Used by emitters that want a specific palette slot regardless
// of any role override.
//
// Layered lookup:
//  1. Vars[name]
//  2. Alacritty[fallbackANSI]
//  3. normHex(fallbackANSI)
func (p *Palette) Var(name, fallbackANSI string) string {
	if hex, ok := p.Vars[name]; ok {
		return hex
	}
	if hex, ok := p.Alacritty.get(fallbackANSI); ok {
		return hex
	}
	return normHex(fallbackANSI)
}

// RoleRaw returns the ORIGINAL palette.toml value for a role (var name or
// #hex), NOT the resolved hex color. Empty string if the role isn't set.
//
// emit_pi.go uses this to preserve "accent = accent" style in pi.json —
// i.e. emit the var name reference, not the flattened hex.
func (p *Palette) RoleRaw(name string) string {
	return p.Roles[name]
}

// --- palette.toml parser ---------------------------------------------------

// parsePaletteTOML reads a palette.toml with the shape:
//
//	[vars]
//	accent = "#549E6A"
//	surface1 = "#23372B"
//
//	[roles]
//	selectedBg = "surface1"
//	borderAccent = "bright"
//	toolPendingBg = "#141F1B"
//
//	[meta]
//	mode = "dark"
//	opacity = 0.85
//	blur = 20
//
// Values are trimmed of surrounding quotes. Vars are normalized to
// #UPPERCASE hex; roles and meta preserve their original scalar form.
//
// Returns (nil, nil, nil, os.ErrNotExist) if the file doesn't exist so
// callers can distinguish "no palette.toml" from "malformed palette.toml".
func parsePaletteTOML(path string) (vars, roles, meta map[string]string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()

	vars = map[string]string{}
	roles = map[string]string{}
	meta = map[string]string{}
	section := ""

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// [section]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, val, ok := splitTOMLKV(line)
		if !ok {
			continue
		}
		switch section {
		case "vars":
			if strings.HasPrefix(val, "#") {
				vars[key] = normHex(val)
			} else {
				// Non-hex vars aren't meaningful in palette.toml semantics,
				// but tolerate them (some themes use `red = "red"` as
				// self-reference; drop those silently).
			}
		case "roles":
			roles[key] = val
		case "meta":
			meta[key] = val
		}
	}
	if err := sc.Err(); err != nil {
		return nil, nil, nil, err
	}
	return vars, roles, meta, nil
}

// splitTOMLKV parses a `key = "value"` or `key = value` line, stripping
// surrounding whitespace, quotes, and any inline `# comment` suffix.
//
// Handles the tricky mix cases:
//   - `key = "value"`                       → (key, value, true)
//   - `key = "#hex"`                        → (key, #hex, true)   [`#` inside quotes]
//   - `key = value # trailing comment`      → (key, value, true)
//   - `key = "value" # trailing comment`    → (key, value, true)   [quoted + inline comment]
//   - `key = #not-a-string`                 → (key, #not-a-string, true) [rare; permissive]
//
// Returns ("", "", false) if the line doesn't have an `=`.
func splitTOMLKV(line string) (key, val string, ok bool) {
	eq := strings.Index(line, "=")
	if eq < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:eq])
	val = strings.TrimSpace(line[eq+1:])
	if len(val) == 0 {
		return key, "", true
	}
	// Detect quoted value first so we don't accidentally strip `#` from
	// inside a quoted string. Consume the quoted region and let any
	// trailing text (including `# comment`) fall off.
	if val[0] == '"' || val[0] == '\'' {
		quote := val[0]
		end := strings.IndexByte(val[1:], quote)
		if end >= 0 {
			return key, val[1 : 1+end], true
		}
		// Unterminated quote — fall through to permissive parse.
	}
	// Unquoted: strip inline comment (space + # + rest).
	if i := strings.Index(val, " #"); i >= 0 {
		val = strings.TrimSpace(val[:i])
	}
	return key, val, true
}

// normHex returns s uppercased and prefixed with `#` if not already.
// Garbage in → garbage out (no validation); parsers upstream should filter.
func normHex(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	if !strings.HasPrefix(s, "#") {
		s = "#" + s
	}
	return strings.ToUpper(s)
}

// --- meta typed accessors --------------------------------------------------

// MetaString returns Meta[key] with quotes stripped; def if absent.
func (p *Palette) MetaString(key, def string) string {
	v, ok := p.Meta[key]
	if !ok || v == "" {
		return def
	}
	return v
}

// MetaFloat parses Meta[key] as a float; def on absent or unparseable.
func (p *Palette) MetaFloat(key string, def float64) float64 {
	v, ok := p.Meta[key]
	if !ok || v == "" {
		return def
	}
	var out float64
	if _, err := fmt.Sscanf(v, "%f", &out); err != nil {
		return def
	}
	return out
}

// MetaInt parses Meta[key] as an int; def on absent or unparseable.
func (p *Palette) MetaInt(key string, def int) int {
	v, ok := p.Meta[key]
	if !ok || v == "" {
		return def
	}
	var out int
	if _, err := fmt.Sscanf(v, "%d", &out); err != nil {
		return def
	}
	return out
}
