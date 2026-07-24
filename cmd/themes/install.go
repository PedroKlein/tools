package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runInstall imports an Omarchy-marketplace theme via git clone.
//
// Accepts either:
//   - a git URL (starts with https:// or git@)
//   - a bare name (resolved by heuristic: github.com/basecamp/omarchy uses
//     the pattern `omarchy-<name>-theme` under various authors; we do not
//     guess owners — a bare name is only accepted if it exists under
//     omarchythemes.com/<name>.git-index or the user has aliased it. For
//     now, bare names error with a clear hint.)
//
// Files copied:
//   alacritty.toml, ghostty.conf (or ghostyy.conf), neovim.lua, btop.theme,
//   preview.png (optional), backgrounds/*, LICENSE.
// Files skipped (Omarchy Linux-only surfaces): chromium.theme, hyprland.conf,
// hyprlock.conf, icons.theme, kitty.conf, mako.ini, swayosd.css, vscode.json,
// walker.css, waybar.css.
func runInstall(args []string) {
	var localPath, explicitName string
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--local":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--local requires a path")
				os.Exit(ExitError)
			}
			localPath = args[i+1]
			i++
		case "--name":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--name requires a value")
				os.Exit(ExitError)
			}
			explicitName = args[i+1]
			i++
		default:
			positional = append(positional, args[i])
		}
	}

	if localPath != "" {
		// Local install path: copy from a directory rather than clone.
		name := explicitName
		if name == "" {
			name = filepath.Base(strings.TrimRight(localPath, "/"))
		}
		if !dirExists(localPath) {
			dieMsg(fmt.Sprintf("not a directory: %s", localPath), ExitError)
		}
		dest := themeDir(name)
		if dirExists(dest) {
			dieMsg(fmt.Sprintf("theme %q already installed at %s", name, dest), ExitAmbiguous)
		}
		if err := copyThemeFiles(localPath, dest); err != nil {
			_ = os.RemoveAll(dest)
			dieMsg(err.Error(), ExitError)
		}
		_ = writeCredits(dest, name, "local:"+localPath)
		if err := runInstallDerive(dest); err != nil {
			fmt.Fprintf(os.Stderr, "warning: derive step failed: %v\n", err)
		}
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "themes install: %s ready (from %s)\n", name, localPath)
		}
		return
	}

	if len(positional) < 1 {
		fmt.Fprintln(os.Stderr, "usage: themes install <url> | themes install --local <path> [--name <name>]")
		os.Exit(ExitError)
	}
	arg := positional[0]
	var (
		url  string
		name string
	)
	if strings.HasPrefix(arg, "http") || strings.HasPrefix(arg, "git@") {
		url = arg
		name = deriveNameFromURL(url)
	} else {
		fmt.Fprintln(os.Stderr, `bare name resolution not implemented in v1;
pass the full git URL, e.g.
    themes install https://github.com/user/omarchy-<name>-theme`)
		os.Exit(ExitError)
	}
	if explicitName != "" {
		name = explicitName
	}
	if name == "" {
		dieMsg("could not derive a theme name from url; pass one with --name", ExitError)
	}

	installedName, err := installFromURL(url, name)
	if err != nil {
		dieMsg(err.Error(), ExitError)
	}
	if !jsonOutput {
		fmt.Fprintf(os.Stderr, "themes install: %s ready.\n", installedName)
		fmt.Fprintf(os.Stderr, "  run: themes set %s\n", installedName)
	}
}

// installFromURL clones url, copies theme files, writes credits, and runs
// the Go derive step. Returns the installed theme name. Does not print;
// callers own their own output. Used by both `themes install <url>` and
// the TUI's `i` prompt.
//
// If a theme with the resolved name already exists, returns ExitAmbiguous
// with a clear error.
func installFromURL(url, name string) (string, error) {
	dest := themeDir(name)
	if dirExists(dest) {
		return "", fmt.Errorf("theme %q already installed at %s", name, dest)
	}
	tmpDir, err := os.MkdirTemp("", "theme-install-"+name+"-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("git", "clone", "--depth", "1", url, tmpDir)
	// Discard stdout/stderr; caller displays whatever progress they want.
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	if err := copyThemeFiles(tmpDir, dest); err != nil {
		_ = os.RemoveAll(dest)
		return "", err
	}
	if err := writeCredits(dest, name, url); err != nil {
		_ = os.RemoveAll(dest)
		return "", err
	}
	if err := runInstallDerive(dest); err != nil {
		_ = os.RemoveAll(dest)
		return "", fmt.Errorf("derive failed: %w", err)
	}
	return name, nil
}
// deriveNameFromURL turns a repo URL into a theme name.
// Rules (mirror Omarchy's convention):
//   - github.com/foo/omarchy-<name>-theme -> <name>
//   - github.com/foo/omarchy-<name>       -> <name>
//   - github.com/foo/<name>-theme         -> <name>
//   - github.com/foo/<name>               -> <name>
func deriveNameFromURL(url string) string {
	// Strip trailing .git and slash.
	u := strings.TrimSuffix(url, ".git")
	u = strings.TrimSuffix(u, "/")
	// Basename.
	slash := strings.LastIndex(u, "/")
	if slash < 0 {
		return ""
	}
	base := u[slash+1:]
	base = strings.TrimPrefix(base, "omarchy-")
	base = strings.TrimSuffix(base, "-theme")
	return base
}

// copyThemeFiles copies the whitelist of upstream files.
func copyThemeFiles(src, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dest, "backgrounds"), 0o755); err != nil {
		return err
	}

	// Files: (upstream-name, dest-name-or-empty-for-same).
	files := []struct{ src, dst string }{
		{"alacritty.toml", ""},
		{"ghostty.conf", ""},
		{"ghostyy.conf", "ghostty.conf"}, // upstream typo (Justikun/osaka-jade)
		{"neovim.lua", ""},
		{"nvim.lua", "neovim.lua"}, // some ports use nvim.lua
		{"btop.theme", ""},
		{"preview.png", ""},
		{"LICENSE", ""},
	}
	found := 0
	for _, f := range files {
		s := filepath.Join(src, f.src)
		d := filepath.Join(dest, ternary(f.dst == "", f.src, f.dst))
		if fileExists(s) && !fileExists(d) {
			if err := copyFile(s, d); err != nil {
				return err
			}
			found++
		}
	}

	// Handle Omarchy's newer colors.toml format: if the theme ships a
	// colors.toml but no alacritty.toml, synthesize alacritty.toml from it.
	if fileExists(filepath.Join(src, "colors.toml")) &&
		!fileExists(filepath.Join(dest, "alacritty.toml")) {
		if err := convertColorsTomlToAlacritty(
			filepath.Join(src, "colors.toml"),
			filepath.Join(dest, "alacritty.toml")); err != nil {
			return fmt.Errorf("convert colors.toml -> alacritty.toml: %w", err)
		}
		// Also emit palette.toml carrying the semantic keys (accent, muted,
		// dark_foreground, surface variants) that colors.toml provides but
		// alacritty.toml cannot express. Enables rich Pi/sketchybar/k9s output.
		if !fileExists(filepath.Join(dest, "palette.toml")) {
			if err := convertColorsTomlToPalette(
				filepath.Join(src, "colors.toml"),
				filepath.Join(dest, "palette.toml")); err != nil {
				return fmt.Errorf("convert colors.toml -> palette.toml: %w", err)
			}
		}
		found++
	}

	if found == 0 {
		return fmt.Errorf("no recognized upstream files found in %s", src)
	}

	// backgrounds/
	if bg := filepath.Join(src, "backgrounds"); dirExists(bg) {
		entries, _ := os.ReadDir(bg)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			s := filepath.Join(bg, e.Name())
			d := filepath.Join(dest, "backgrounds", e.Name())
			_ = copyFile(s, d)
		}
	}
	return nil
}

// convertColorsTomlToAlacritty translates Omarchy's colors.toml into the
// alacritty.toml layout the derive tool understands.
//
// Handles two formats:
//   (a) legacy: color0..color15 flat keys                (older themes)
//   (b) newer:  named keys (red, green, blue, ..., bright_red, ...) (basecamp/omarchy monorepo)
//
// If (a) is present it wins. Otherwise fall back to (b).
func convertColorsTomlToAlacritty(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	kv := parseFlatToml(string(b))
	get := func(key, fallback string) string {
		if v, ok := kv[key]; ok && v != "" {
			return v
		}
		return fallback
	}

	// ANSI palette (16 colors).
	type pair struct{ name, k string }
	// Order matches xterm ANSI 0..7 (normal), 8..15 (bright).
	normal := []pair{
		// key `black` isn't in the named scheme; use `muted` as a dark grey.
		{"black", ""}, {"red", "red"}, {"green", "green"}, {"yellow", "yellow"},
		{"blue", "blue"}, {"magenta", "magenta"}, {"cyan", "cyan"}, {"white", ""},
	}

	// Detect which format we have.
	hasLegacy := kv["color0"] != ""

	resolveNormal := func(i int, p pair) string {
		if hasLegacy {
			return get(fmt.Sprintf("color%d", i), "#000000")
		}
		switch p.name {
		case "black":
			return get("muted", get("selection", get("dark_background", "#000000")))
		case "white":
			return get("foreground", "#ffffff")
		default:
			return get(p.k, "#000000")
		}
	}
	resolveBright := func(i int, p pair) string {
		if hasLegacy {
			return get(fmt.Sprintf("color%d", i+8), "#000000")
		}
		switch p.name {
		case "black":
			return get("lighter_background", get("muted", "#000000"))
		case "white":
			return get("bright_foreground", get("foreground", "#ffffff"))
		default:
			return get("bright_"+p.k, get(p.k, "#000000"))
		}
	}

	var out strings.Builder
	fmt.Fprintln(&out, "# Synthesized from upstream colors.toml by 'themes install'.")
	fmt.Fprintln(&out, "[colors]")
	fmt.Fprintln(&out, "[colors.primary]")
	fmt.Fprintf(&out, "background = '%s'\n", get("background", "#000000"))
	fmt.Fprintf(&out, "foreground = '%s'\n", get("foreground", "#ffffff"))
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "[colors.normal]")
	for i, p := range normal {
		fmt.Fprintf(&out, "%-7s = \"%s\"\n", p.name, resolveNormal(i, p))
	}
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "[colors.bright]")
	for i, p := range normal {
		fmt.Fprintf(&out, "%-7s = \"%s\"\n", p.name, resolveBright(i, p))
	}
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "[colors.cursor]")
	fmt.Fprintf(&out, "cursor = \"%s\"\n", get("cursor", get("foreground", "#ffffff")))
	fmt.Fprintf(&out, "text   = \"%s\"\n", get("selection_foreground", get("background", "#000000")))
	return os.WriteFile(dst, []byte(out.String()), 0o644)
}

// parseFlatToml reads a flat TOML file with key = "value" or key = '#hex' lines.
// Ignores sections and comments. Enough for Omarchy's colors.toml.
func parseFlatToml(s string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `"'`)
		out[key] = val
	}
	return out
}

// convertColorsTomlToPalette emits a palette.toml with semantic vars + roles
// distilled from Omarchy's colors.toml. This is what feeds the enriched
// output for Pi, k9s, delta, sketchybar (FG_MUTED, SURFACE, ACCENT_BRIGHT, etc.).
func convertColorsTomlToPalette(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	kv := parseFlatToml(string(b))
	get := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := kv[k]; ok && v != "" {
				return v
			}
		}
		return ""
	}

	// Assemble vars from whatever colors.toml provides.
	type kvp struct{ k, v string }
	vars := []kvp{}
	addVar := func(k, v string) {
		if v != "" {
			vars = append(vars, kvp{k, v})
		}
	}
	addVar("bg", get("background"))
	addVar("mantle", get("darker_background", "dark_background", "background"))
	addVar("surface0", get("dark_background"))
	addVar("surface1", get("lighter_background", "selection"))
	addVar("surface2", get("lighter_background"))
	addVar("accent", get("accent"))
	addVar("bright", get("bright_foreground", "foreground"))
	addVar("muted", get("dark_foreground", "muted"))
	addVar("fg", get("foreground"))
	addVar("white", get("bright_foreground", "foreground"))
	addVar("red", get("red"))
	addVar("redBright", get("bright_red", "red"))
	addVar("yellow", get("yellow"))
	addVar("cyan", get("cyan"))
	addVar("cyanMuted", get("bright_cyan", "cyan"))
	addVar("blue", get("blue"))
	addVar("purple", get("magenta"))
	addVar("pink", get("bright_magenta", "magenta"))
	addVar("jade", get("green"))
	addVar("teal", get("cyan"))

	// Roles that let derive downstream produce rich output.
	roles := []kvp{
		{"accent", "accent"},
		{"border", "surface1"},
		{"borderAccent", "accent"},
		{"borderMuted", "surface1"},
		{"success", "bright"},
		{"error", "red"},
		{"warning", "yellow"},
		{"muted", "muted"},
		{"dim", "muted"},
		{"selectedBg", "surface1"},
		{"userMessageBg", "surface0"},
		{"toolPendingBg", "surface0"},
		{"toolSuccessBg", "surface0"},
		{"toolErrorBg", "surface0"},
		{"toolOutput", "muted"},
		{"mdHeading", "yellow"},
		{"mdLink", "cyan"},
		{"mdLinkUrl", "muted"},
		{"mdCode", "bright"},
		{"mdCodeBlock", "fg"},
		{"mdCodeBlockBorder", "muted"},
		{"mdQuote", "muted"},
		{"mdQuoteBorder", "muted"},
		{"mdHr", "muted"},
		{"mdListBullet", "accent"},
		{"toolDiffAdded", "bright"},
		{"toolDiffRemoved", "red"},
		{"toolDiffContext", "muted"},
		{"syntaxComment", "muted"},
		{"syntaxKeyword", "purple"},
		{"syntaxFunction", "yellow"},
		{"syntaxVariable", "cyanMuted"},
		{"syntaxString", "bright"},
		{"syntaxNumber", "cyan"},
		{"syntaxType", "accent"},
		{"syntaxOperator", "fg"},
		{"syntaxPunctuation", "muted"},
		{"thinkingText", "muted"},
		{"thinkingLow", "teal"},
		{"thinkingMedium", "accent"},
		{"thinkingHigh", "bright"},
		{"thinkingXhigh", "cyan"},
	}

	var out strings.Builder
	fmt.Fprintln(&out, "# Auto-generated from upstream colors.toml by 'themes install'.")
	fmt.Fprintln(&out, "# Hand-editing is fine; themes derive will preserve manual changes if")
	fmt.Fprintln(&out, "# re-run against the same source colors.toml.")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "[vars]")
	varNames := map[string]bool{}
	for _, v := range vars {
		if varNames[v.k] {
			continue
		}
		varNames[v.k] = true
		fmt.Fprintf(&out, "%-10s = %q\n", v.k, strings.ToUpper(v.v))
	}
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "[roles]")
	for _, r := range roles {
		// Only emit roles whose fallback var was actually defined.
		if r.v != "" && !strings.HasPrefix(r.v, "#") && !varNames[r.v] {
			continue
		}
		fmt.Fprintf(&out, "%-19s = %q\n", r.k, r.v)
	}
	return os.WriteFile(dst, []byte(out.String()), 0o644)
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func writeCredits(dest, name, url string) error {
	body := fmt.Sprintf(`# %s — credits & provenance

## Upstream

- **Repo:** %s
- **Imported:** %s (via 'themes install')

Files taken verbatim from upstream: alacritty.toml, ghostty.conf, neovim.lua,
btop.theme, preview.png (if provided), backgrounds/, LICENSE.

## Files derived (not from upstream)

palette.toml (author manually to enrich pi/k9s/delta),
plus the 13 files emitted by ~/.config/themes/.bin/theme-derive:
tmux.conf, starship.toml, k9s.yaml, television.toml, lazygit.yml, gh-dash.yml,
opencode.name, bat.tmTheme, delta.gitconfig, fzf.sh, zsh-syntax-highlight.zsh,
sketchybar.sh, pi.json.
`, name, url, time.Now().UTC().Format("2006-01-02"))
	return os.WriteFile(filepath.Join(dest, "CREDITS.md"), []byte(body), 0o644)
}

// runInstallDerive runs the palette-based derive step for a freshly
// installed theme. Wraps deriveTheme (defined in derive.go) with the
// error-formatting used by the install flow.
func runInstallDerive(themeAbsDir string) error {
	_, _, err := deriveTheme(themeAbsDir)
	return err
}

// Retained for future use in bare-name resolution.
var _ = http.StatusOK
