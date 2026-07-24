package palette

import (
	"encoding/json"
	"fmt"
	"io"
)

// piSchema is the pi-mono theme JSON schema URL. Pinned to `main` to match
// Python theme-derive.
const piSchema = "https://raw.githubusercontent.com/badlogic/pi-mono/main/packages/coding-agent/src/modes/interactive/theme/theme-schema.json"

// piVarFallback maps a Pi var slot to its ANSI fallback name.
type piVarFallback struct {
	Name         string
	FallbackANSI string
}

// piVarFallbacks is the ordered list of Pi's 20 named vars with ANSI
// fallbacks. Order is preserved to keep JSON output stable across ports.
var piVarFallbacks = []piVarFallback{
	{"bg", "bg"}, {"mantle", "bg"}, {"surface0", "b_black"},
	{"surface1", "b_black"}, {"surface2", "b_black"},
	{"accent", "green"}, {"bright", "b_green"}, {"muted", "b_black"},
	{"fg", "fg"}, {"white", "white"},
	{"red", "red"}, {"redBright", "b_red"}, {"yellow", "yellow"},
	{"cyan", "cyan"}, {"cyanMuted", "b_cyan"},
	{"blue", "blue"}, {"purple", "magenta"}, {"pink", "magenta"},
	{"jade", "green"}, {"teal", "blue"},
}

// piRoles is the ordered list of Pi's color roles, with their ANSI
// fallbacks used when neither palette.roles nor palette.vars supplies them.
var piRoles = []piVarFallback{
	{"accent", "green"}, {"border", "blue"}, {"borderAccent", "b_green"},
	{"borderMuted", "b_black"}, {"success", "b_green"}, {"error", "red"},
	{"warning", "yellow"}, {"muted", "b_black"}, {"dim", "b_black"},
	{"thinkingText", "b_black"},
	{"selectedBg", "b_black"}, {"userMessageBg", "b_black"},
	{"customMessageBg", "b_black"}, {"customMessageLabel", "magenta"},
	{"toolPendingBg", "b_black"}, {"toolSuccessBg", "b_black"},
	{"toolErrorBg", "b_black"}, {"toolOutput", "b_black"},
	{"mdHeading", "yellow"}, {"mdLink", "cyan"}, {"mdLinkUrl", "b_black"},
	{"mdCode", "b_green"}, {"mdCodeBlock", "fg"}, {"mdCodeBlockBorder", "b_black"},
	{"mdQuote", "b_black"}, {"mdQuoteBorder", "b_black"},
	{"mdHr", "b_black"}, {"mdListBullet", "green"},
	{"toolDiffAdded", "b_green"}, {"toolDiffRemoved", "red"},
	{"toolDiffContext", "b_black"},
	{"syntaxComment", "b_black"}, {"syntaxKeyword", "magenta"},
	{"syntaxFunction", "yellow"}, {"syntaxVariable", "b_cyan"},
	{"syntaxString", "b_green"}, {"syntaxNumber", "cyan"},
	{"syntaxType", "green"}, {"syntaxOperator", "fg"},
	{"syntaxPunctuation", "b_black"},
	{"thinkingOff", "b_black"}, {"thinkingMinimal", "b_black"},
	{"thinkingLow", "blue"}, {"thinkingMedium", "green"},
	{"thinkingHigh", "b_green"}, {"thinkingXhigh", "cyan"},
	{"bashMode", "yellow"},
}

// piInherit lists role slots Pi treats as "inherit" — emitted as empty
// string in the JSON.
var piInherit = []string{"text", "userMessageText", "customMessageText", "toolTitle"}

// piExport pairs an export slot with the var it should reference and a
// final ANSI fallback for themes without palette.toml.
type piExport struct {
	ExportName   string
	VarName      string
	FallbackANSI string
}

var piExports = []piExport{
	{"pageBg", "mantle", "bg"},
	{"cardBg", "surface0", "b_black"},
	{"infoBg", "surface1", "b_black"},
}

// piThemeJSON is the wire shape of pi.json. Uses ordered field names via
// json struct tags; Go's encoding/json preserves struct field order.
//
// `colors` and `export` are ordered maps: we build them in the fixed piRoles
// / piExports order and encode via encoding/json. Since Go maps randomize
// iteration, we hand-roll the JSON with a small ordered-writer helper to
// keep output byte-identical to Python.
type piThemeJSON struct {
	Schema string `json:"$schema"`
	Name   string `json:"name"`
	// vars/colors/export handled manually to preserve field order.
}

// EmitPi writes a pi.json theme. Uses named var references in [colors] when
// a palette.toml var maps 1:1 to a role's hex value; falls back to raw hex.
//
// Output uses tab indent to match the user's existing pi.json style.
func EmitPi(w io.Writer, p *Palette) error {
	// Build the vars block first, since roles reference-scan back through
	// it to recover var names for the "accent = accent" style pi.json.
	vars := make(map[string]string, len(piVarFallbacks))
	for _, v := range piVarFallbacks {
		vars[v.Name] = p.Var(v.Name, v.FallbackANSI)
	}

	// roleValue emits either the var name (if the role's hex matches an
	// entry in vars) or the raw hex.
	//
	// Reverse-scan iterates piVarFallbacks in insertion order (NOT the
	// randomized map iteration) so multi-var themes (rose-pine has
	// `accent` and `blue` sharing a hex) match Python's insertion-ordered
	// dict behavior byte-for-byte.
	roleValue := func(roleName, fallbackANSI string) string {
		raw, ok := p.Roles[roleName]
		if !ok {
			return p.Role(roleName, fallbackANSI)
		}
		var hex string
		if len(raw) > 0 && raw[0] == '#' {
			hex = normHex(raw)
		} else if v, ok := p.Vars[raw]; ok {
			hex = v
		} else {
			hex = p.Role(roleName, fallbackANSI)
		}
		for _, v := range piVarFallbacks {
			if vars[v.Name] == hex {
				return v.Name
			}
		}
		return hex
	}

	// Ordered JSON build. encoding/json doesn't preserve map order, so we
	// write the object manually with fmt to guarantee stable byte output
	// matching Python (which uses insertion-ordered dicts).
	//
	// json.Marshal is used per-string-value to get correct quoting/escaping.
	//
	// Format: tab-indented, 1-tab nesting, keys sorted per Python insertion
	// order (Python 3.7+ dicts are insertion-ordered).
	fmt.Fprintf(w, "{\n\t\"$schema\": %s,\n\t\"name\": %s,\n",
		jsonString(piSchema), jsonString(p.Name))

	// vars block
	fmt.Fprint(w, "\t\"vars\": {\n")
	for i, v := range piVarFallbacks {
		sep := ","
		if i == len(piVarFallbacks)-1 {
			sep = ""
		}
		fmt.Fprintf(w, "\t\t%s: %s%s\n", jsonString(v.Name), jsonString(vars[v.Name]), sep)
	}
	fmt.Fprint(w, "\t},\n")

	// colors block
	fmt.Fprint(w, "\t\"colors\": {\n")
	total := len(piRoles) + len(piInherit)
	idx := 0
	for _, r := range piRoles {
		idx++
		sep := ","
		if idx == total {
			sep = ""
		}
		fmt.Fprintf(w, "\t\t%s: %s%s\n", jsonString(r.Name), jsonString(roleValue(r.Name, r.FallbackANSI)), sep)
	}
	for _, name := range piInherit {
		idx++
		sep := ","
		if idx == total {
			sep = ""
		}
		fmt.Fprintf(w, "\t\t%s: %s%s\n", jsonString(name), jsonString(""), sep)
	}
	fmt.Fprint(w, "\t},\n")

	// export block
	fmt.Fprint(w, "\t\"export\": {\n")
	for i, e := range piExports {
		sep := ","
		if i == len(piExports)-1 {
			sep = ""
		}
		// If palette.roles has an explicit override for this export name,
		// use it (var name preserved when possible). Otherwise fall back to
		// the paired var name if defined, else the ANSI fallback.
		val := piExportValue(p, e, vars)
		fmt.Fprintf(w, "\t\t%s: %s%s\n", jsonString(e.ExportName), jsonString(val), sep)
	}
	fmt.Fprint(w, "\t}\n")
	fmt.Fprint(w, "}\n")
	return nil
}

// piExportValue resolves one piExports entry against the palette:
//
//  1. Explicit [roles] entry with matching name → use it (var name if the
//     hex reverse-matches, else the raw hex).
//  2. Otherwise the paired var (e.g. `mantle` for `pageBg`) if defined
//     in the palette (Python emits the var name).
//  3. Otherwise the ANSI fallback.
func piExportValue(p *Palette, e piExport, vars map[string]string) string {
	if raw, ok := p.Roles[e.ExportName]; ok {
		var hex string
		if len(raw) > 0 && raw[0] == '#' {
			hex = normHex(raw)
		} else if v, ok := p.Vars[raw]; ok {
			hex = v
		} else {
			hex = p.Role(e.ExportName, e.FallbackANSI)
		}
		for _, v := range piVarFallbacks {
			if vars[v.Name] == hex {
				return v.Name
			}
		}
		return hex
	}
	if _, ok := vars[e.VarName]; ok {
		return e.VarName
	}
	if hex, ok := p.Alacritty.get(e.FallbackANSI); ok {
		return hex
	}
	return normHex(e.FallbackANSI)
}

// jsonString wraps encoding/json to produce a properly-escaped, quoted
// JSON string literal. Bare strings.ReplaceAll would miss unicode escapes
// and control chars; delegating to encoding/json is safer.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
