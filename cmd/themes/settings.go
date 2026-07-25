package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
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
	if _, err := deriveThemeV4(m.themeDir); err != nil {
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
//
// v4 note: settings live inside theme.json (effects.opacity, effects.blur,
// macos.appearance) instead of the v3 palette.toml [meta] block. This
// file exposes a map[string]string API so the settings-pane widgets
// don't have to know about the storage format. P5.4 completes the
// unknown-key-preservation contract; P1.11 lands the minimum needed
// for the palette.toml deletion sweep.

// loadMeta reads settings-relevant fields from <themeDir>/theme.json.
// Returns an empty map when theme.json is absent or unparseable.
func loadMeta(themeDir string) map[string]string {
	out := map[string]string{}
	th, err := palette.Load(themeDir)
	if err != nil {
		return out
	}
	if th.Effects.Opacity > 0 && th.Effects.Opacity < 1 {
		out["opacity"] = fmt.Sprintf("%g", th.Effects.Opacity)
	}
	if th.Effects.Blur > 0 {
		out["blur"] = fmt.Sprintf("%d", int(th.Effects.Blur))
	}
	if th.Macos.Appearance != "" && th.Macos.Appearance != th.Appearance {
		out["mode"] = th.Macos.Appearance
	}
	if th.Macos.Accent != "" {
		out["accent_preset"] = th.Macos.Accent
	}
	if th.Macos.Highlight != "" {
		out["highlight_hex"] = th.Macos.Highlight
	}
	return out
}

// writeMeta merges updates into <themeDir>/theme.json's effects/macos
// blocks. Values of "" delete the corresponding field.
//
// Uses a map[string]any round-trip so unknown top-level keys survive
// (partial P5.4 — field ordering may reshuffle since encoding/json
// alphabetizes maps).
func writeMeta(themeDir string, updates map[string]string) error {
	path := filepath.Join(themeDir, "theme.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("writeMeta: read theme.json: %w", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("writeMeta: parse theme.json: %w", err)
	}
	effects := ensureObject(doc, "effects")
	macos := ensureObject(doc, "macos")

	for k, v := range updates {
		switch k {
		case "opacity":
			if v == "" {
				delete(effects, "opacity")
				continue
			}
			var f float64
			fmt.Sscanf(v, "%f", &f)
			effects["opacity"] = f
		case "blur":
			if v == "" {
				delete(effects, "blur")
				continue
			}
			var n int
			fmt.Sscanf(v, "%d", &n)
			effects["blur"] = n
		case "mode":
			if v == "" || v == "auto" {
				delete(macos, "appearance")
				continue
			}
			macos["appearance"] = v
		case "accent_preset":
			if v == "" {
				delete(macos, "accent")
				continue
			}
			macos["accent"] = v
		case "highlight_hex":
			if v == "" {
				delete(macos, "highlight")
				continue
			}
			macos["highlight"] = v
		}
	}

	// Drop empty objects.
	if len(effects) == 0 {
		delete(doc, "effects")
	}
	if len(macos) == 0 {
		delete(doc, "macos")
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return writeFileAtomic(path, out, 0o644)
}

// ensureObject returns doc[key] as a map, creating an empty one if absent.
func ensureObject(doc map[string]any, key string) map[string]any {
	if existing, ok := doc[key].(map[string]any); ok {
		return existing
	}
	m := map[string]any{}
	doc[key] = m
	return m
}

// --- meta getter helpers --------------------------------------------------

func metaFloat(meta map[string]string, key string, def float64) float64 {
	if v, ok := meta[key]; ok {
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
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

// isLightTheme reads <themeDir>/theme.json and returns true when the
// background reads as light (YIQ > 128). Falls back to false when
// theme.json is missing/invalid, which keeps the ghostty translucency
// defaults on the dark-theme code path.
func isLightTheme(themeDir string) bool {
	th, err := palette.Load(themeDir)
	if err != nil {
		return false
	}
	return palette.IsLight(th.Palette.Semantic.Bg)
}

// roundHundredths rounds x to two decimal places (avoid float display drift).
func roundHundredths(x float64) float64 {
	return float64(int(x*100+0.5)) / 100
}
