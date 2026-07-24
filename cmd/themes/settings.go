package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// runSettingsInteractive opens the per-current-theme settings sub-TUI.
//
// Adjustable knobs (all persist to the theme's palette.toml [meta] block):
//   • opacity   0.5-1.0  step 0.05  → ghostty background-opacity
//   • blur      0-30     step 2     → ghostty background-blur
//   • mode      auto/light/dark     → .macos.json mode (overrides YIQ detection)
//
// Live-apply: every change writes the new [meta] value, re-derives the
// theme, and runs the reload orchestrator so ghostty picks up the new
// translucency and macOS updates its accent (via .macos.json).
//
// Escape returns to the picker WITHOUT reverting; changes persist. This
// matches the picker's own "commit-on-change" model.
func runSettingsInteractive() {
	s, err := LoadState()
	if err != nil {
		dieMsg(err.Error(), ExitError)
	}
	if s.Theme == "" {
		dieMsg("no theme active", ExitNotFound)
	}
	dir := themeDir(s.Theme)
	meta := loadMeta(dir)

	m := &settingsModel{
		themeName: s.Theme,
		themeDir:  dir,
		opacity:   metaFloat(meta, "opacity", defaultOpacity(dir)),
		blur:      metaInt(meta, "blur", defaultBlur(dir)),
		mode:      metaString(meta, "mode", "auto"),
	}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "themes:", err)
		os.Exit(ExitError)
	}
	if m.err != nil {
		fmt.Fprintln(os.Stderr, "themes:", m.err)
		os.Exit(ExitError)
	}
}

// settingsModel is the Bubbletea state.
type settingsModel struct {
	themeName string
	themeDir  string
	opacity   float64
	blur      int
	mode      string // "auto", "light", "dark"
	focus     int    // 0=opacity, 1=blur, 2=mode
	err       error
	quitting  bool

	// reloadCancel cancels an in-flight background reload before starting
	// a new one. Prevents overlapping reloads on rapid key-repeat: each
	// adjust() cancels the prior reload's context and spawns a fresh one
	// so we never accumulate goroutines nor race on the shared
	// THEME_LIVE_APPLY env var.
	reloadCancel context.CancelFunc
}

func (m *settingsModel) Init() tea.Cmd { return nil }

func (m *settingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "q", "ctrl+c", "esc":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.focus > 0 {
			m.focus--
		}
	case "down", "j":
		if m.focus < 2 {
			m.focus++
		}
	case "left", "h":
		m.adjust(-1)
	case "right", "l":
		m.adjust(+1)
	case "enter":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// adjust moves the focused field one step in dir. Applies live: writes
// [meta] and re-derives.
func (m *settingsModel) adjust(dir int) {
	switch m.focus {
	case 0:
		next := m.opacity + float64(dir)*0.05
		if next < 0.5 {
			next = 0.5
		}
		if next > 1.0 {
			next = 1.0
		}
		m.opacity = roundHundredths(next)
	case 1:
		next := m.blur + dir*2
		if next < 0 {
			next = 0
		}
		if next > 30 {
			next = 30
		}
		m.blur = next
	case 2:
		modes := []string{"auto", "light", "dark"}
		cur := 0
		for i, mode := range modes {
			if mode == m.mode {
				cur = i
				break
			}
		}
		next := (cur + dir + len(modes)) % len(modes)
		m.mode = modes[next]
	}
	// Persist + re-derive + reload. Silently swallow errors \u2014 they'll
	// surface in the picker's next repaint if state got wedged.
	if err := writeMeta(m.themeDir, map[string]string{
		"opacity": fmt.Sprintf("%g", m.opacity),
		"blur":    fmt.Sprintf("%d", m.blur),
		"mode":    modeMetaValue(m.mode),
	}); err != nil {
		m.err = err
		return
	}
	if _, _, err := deriveTheme(m.themeDir); err != nil {
		m.err = err
		return
	}
	// Cancel any prior in-flight reload before spawning a new one.
	// Guards against key-repeat storms and prevents THEME_LIVE_APPLY
	// races between overlapping goroutines.
	if m.reloadCancel != nil {
		m.reloadCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.reloadCancel = cancel
	go func() {
		defer cancel()
		_ = runReloadHookCtx(ctx, m.themeDir, nil, true)
	}()
}

func (m *settingsModel) View() string {
	if m.quitting {
		return ""
	}
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#63B07A")).
		Render("Settings \u2014 " + m.themeName)

	rows := []string{
		settingRow("Opacity", fmt.Sprintf("%.2f", m.opacity), sliderBar(m.opacity, 0.5, 1.0, 20), m.focus == 0),
		settingRow("Blur", fmt.Sprintf("%d", m.blur), sliderBar(float64(m.blur), 0, 30, 20), m.focus == 1),
		settingRow("Mode", m.mode, modeChooser(m.mode), m.focus == 2),
	}

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#509475")).
		Padding(1, 2).
		Width(60).
		Render(body)

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#627A6C")).
		Render("\u2191\u2193 select \u00b7 \u2190\u2192 adjust \u00b7 \u23ce/esc back")

	return lipgloss.JoinVertical(lipgloss.Left, title, "", box, "", help)
}

// settingRow renders one setting: label, value, visualizer.
func settingRow(label, value, viz string, focused bool) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#627A6C"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#C1C497"))
	if focused {
		labelStyle = labelStyle.Foreground(lipgloss.Color("#63B07A")).Bold(true)
		valueStyle = valueStyle.Foreground(lipgloss.Color("#63B07A")).Bold(true)
	}
	prefix := "  "
	if focused {
		prefix = "\u25b8 "
	}
	return fmt.Sprintf("%s%s  %s  %s",
		prefix,
		labelStyle.Render(fmt.Sprintf("%-8s", label)),
		valueStyle.Render(fmt.Sprintf("%-6s", value)),
		viz,
	)
}

// sliderBar renders a horizontal fill bar. width is the total bar width in
// characters. Uses \u2588 for filled cells, \u2591 for empty.
func sliderBar(value, min, max float64, width int) string {
	if max == min {
		return strings.Repeat("\u2591", width)
	}
	ratio := (value - min) / (max - min)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(float64(width) * ratio)
	return strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", width-filled)
}

// modeChooser renders auto/light/dark as three pill buttons with the
// current one highlighted.
func modeChooser(current string) string {
	pills := []string{"auto", "light", "dark"}
	rendered := make([]string, 0, len(pills))
	for _, p := range pills {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#627A6C"))
		if p == current {
			style = style.Foreground(lipgloss.Color("#63B07A")).Bold(true).Underline(true)
		}
		rendered = append(rendered, style.Render(p))
	}
	return strings.Join(rendered, " \u00b7 ")
}

// modeMetaValue maps the UI's "auto" state to empty string so we don't
// override YIQ detection at the .macos.json emitter. "light" and "dark"
// are stored verbatim.
func modeMetaValue(mode string) string {
	if mode == "auto" {
		return ""
	}
	return mode
}

// --- meta persistence -----------------------------------------------------

// loadMeta reads the [meta] block from <themeDir>/palette.toml.
// Returns an empty map if palette.toml is absent.
func loadMeta(themeDir string) map[string]string {
	data, err := os.ReadFile(filepath.Join(themeDir, "palette.toml"))
	if err != nil {
		return map[string]string{}
	}
	return extractMetaSection(string(data))
}

// extractMetaSection is a minimal TOML slicer that pulls out the [meta]
// section as key\u2192value pairs. Duplicates parsePaletteTOML's logic to
// avoid coupling to the internal package.
func extractMetaSection(s string) map[string]string {
	out := map[string]string{}
	inMeta := false
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			inMeta = strings.Trim(t, "[]") == "meta"
			continue
		}
		if !inMeta {
			continue
		}
		eq := strings.Index(t, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(t[:eq])
		val := strings.TrimSpace(t[eq+1:])
		val = strings.Trim(val, `"'`)
		out[key] = val
	}
	return out
}

// writeMeta patches the [meta] block in <themeDir>/palette.toml.
// MERGES with existing keys instead of replacing them, so unrelated
// meta (e.g. accent_preset overrides, custom user knobs) survive edits.
//
// Values whose value is "" are removed from the file (lets "mode=auto"
// clear the override cleanly).
//
// Behavior: read the existing [meta] section into a map, apply updates
// on top (empty values delete), then re-emit with keys sorted for stable
// output. Non-meta sections pass through byte-for-byte. Inline comments
// inside [meta] are lost — [meta] is a scalar bag, not a place for prose.
func writeMeta(themeDir string, updates map[string]string) error {
	path := filepath.Join(themeDir, "palette.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		merged := map[string]string{}
		for k, v := range updates {
			if v != "" {
				merged[k] = v
			}
		}
		return writeFileAtomic(path, []byte(renderMetaSection(merged)), 0o644)
	}

	// Merge existing [meta] with updates.
	merged := extractMetaSection(string(data))
	for k, v := range updates {
		if v == "" {
			delete(merged, k)
		} else {
			merged[k] = v
		}
	}

	lines := strings.Split(string(data), "\n")
	var out []string
	inMeta := false
	metaWritten := false

	writeMetaBody := func() {
		out = append(out, strings.TrimRight(renderMetaSection(merged), "\n"))
		metaWritten = true
	}

	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
			section := strings.Trim(trim, "[]")
			if section == "meta" {
				inMeta = true
				writeMetaBody()
				continue
			}
			if inMeta {
				inMeta = false
			}
			out = append(out, line)
			continue
		}
		if inMeta {
			continue
		}
		if i == len(lines)-1 && line == "" && len(out) > 0 && out[len(out)-1] == "" {
			continue
		}
		out = append(out, line)
	}
	if !metaWritten {
		out = append(out, "")
		writeMetaBody()
	}
	return writeFileAtomic(path, []byte(strings.Join(out, "\n")), 0o644)
}

// renderMetaSection emits `[meta]` + `key = value` per entry in sorted
// key order so output is stable across writes.
func renderMetaSection(meta map[string]string) string {
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("[meta]\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s = %s\n", k, formatMetaValue(meta[k]))
	}
	return b.String()
}

// formatMetaValue chooses a quoting/type for a meta value:
//   • numeric strings pass through unquoted
//   • everything else gets double-quoted
func formatMetaValue(v string) string {
	if _, err := parseNumeric(v); err == nil {
		return v
	}
	return `"` + v + `"`
}

// parseNumeric is a lightweight check for "looks like a number" without
// pulling strconv into this file's imports twice.
func parseNumeric(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// --- meta getter helpers --------------------------------------------------

func metaFloat(meta map[string]string, key string, def float64) float64 {
	if v, ok := meta[key]; ok {
		if f, err := parseNumeric(v); err == nil {
			return f
		}
	}
	return def
}

func metaInt(meta map[string]string, key string, def int) int {
	if v, ok := meta[key]; ok {
		var out int
		if _, err := fmt.Sscanf(v, "%d", &out); err == nil {
			return out
		}
	}
	return def
}

func metaString(meta map[string]string, key, def string) string {
	if v, ok := meta[key]; ok && v != "" {
		return v
	}
	return def
}

// defaultOpacity / defaultBlur return the derive-layer defaults so an
// unset [meta] shows the same value the theme actually uses.
//
// Both peek at the theme's alacritty.toml to detect light vs dark, then
// use the same magic numbers as internal/palette/emit.go's EmitGhostty.
func defaultOpacity(themeDir string) float64 {
	if isLightTheme(themeDir) {
		return 0.97
	}
	return 0.85
}

func defaultBlur(themeDir string) int {
	if isLightTheme(themeDir) {
		return 8
	}
	return 20
}

// isLightTheme is a rough duplicate of internal/palette.IsLight. Kept
// package-local to avoid a cyclic import when settings.go is in cmd/themes
// but IsLight lives in internal/palette. Small YIQ formula.
func isLightTheme(themeDir string) bool {
	// Read alacritty.toml primary background.
	data, err := os.ReadFile(filepath.Join(themeDir, "alacritty.toml"))
	if err != nil {
		return false
	}
	bg := "#000000"
	inPrimary := false
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			s := strings.TrimPrefix(strings.Trim(t, "[]"), "colors.")
			inPrimary = s == "primary"
			continue
		}
		if inPrimary && strings.HasPrefix(t, "background") {
			eq := strings.Index(t, "=")
			if eq >= 0 {
				v := strings.TrimSpace(t[eq+1:])
				v = strings.Trim(v, `"'`)
				bg = v
			}
			break
		}
	}
	// YIQ brightness.
	s := strings.TrimPrefix(bg, "#")
	if len(s) != 6 {
		return false
	}
	var r, g, b int
	fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b)
	return (r*299+g*587+b*114)/1000 > 128
}

// roundHundredths rounds x to two decimal places (avoid float display drift).
func roundHundredths(x float64) float64 {
	return float64(int(x*100+0.5)) / 100
}
