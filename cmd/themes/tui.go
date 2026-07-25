package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

// runTUI is the interactive theme picker.
//
// Layout:
//
//	┌ Themes (n) ────────────┐ ┌ Preview ─────────────────────────┐
//	│ ▸ osaka-jade           │ │ ██ ██ ██ ██ ██ ██ ██ ██          │
//	│   tokyonight           │ │ ██ ██ ██ ██ ██ ██ ██ ██          │
//	│   ...                  │ │                                  │
//	│                        │ │ Author: from CREDITS             │
//	└────────────────────────┘ └──────────────────────────────────┘
//	 ↑↓ navigate  ⏎ confirm  esc revert  L toggle-live  q quit
//
// Live-apply on scroll (P3.5) is added in a follow-up commit.
func runTUI(_ []string) {
	runTUIWith("")
}

// runTUIWith launches the picker with an optional focus theme (cursor jumps
// to that theme if present, else falls back to the currently-active theme).
// Used by the `i` install flow to focus the newly installed theme.
func runTUIWith(focusTheme string) {
	themes, err := ListThemes()
	if err != nil {
		fmt.Fprintln(os.Stderr, "themes:", err)
		os.Exit(ExitError)
	}
	if len(themes) == 0 {
		fmt.Fprintln(os.Stderr, "no themes installed; run: themes install <url>")
		os.Exit(ExitNotFound)
	}
	s := activeState()
	initial := 0
	target := focusTheme
	if target == "" {
		target = s.Theme
	}
	for i, t := range themes {
		if t.Name == target {
			initial = i
			break
		}
	}
	m := &pickerModel{
		themes:    themes,
		cursor:    initial,
		startedAt: s.Theme,
		liveApply: true,
	}
	m.reloadStyles()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "theme: TUI error:", err)
		os.Exit(ExitError)
	}
	// Commit / revert handled inside model.
	if m.err != nil {
		fmt.Fprintln(os.Stderr, "themes:", m.err)
		os.Exit(ExitError)
	}
	if m.openWallpaperAfter {
		runWallpaperInteractive()
	}
	if m.openInstallAfter {
		installed := runInstallInteractive()
		// Re-launch the picker focused on the newly installed theme.
		runTUIWith(installed)
	}
	if m.openSettingsAfter {
		runSettingsInteractive()
		runTUIWith("")
	}
}

type pickerModel struct {
	themes    []ThemeInfo
	cursor    int
	startedAt string // theme active when picker opened, for Esc revert
	liveApply bool
	err       error
	quitting  bool
	// openWallpaperAfter, when true, signals runInteractive to launch the
	// wallpaper subpicker after the picker's tea.Program exits. Set from
	// the `w` key handler. tea.Sequence(tea.Quit, ...) doesn't work because
	// Quit terminates the event loop before the next Cmd runs.
	openWallpaperAfter bool
	// openInstallAfter is the same tunnel pattern for the `i` install
	// prompt. On completion the picker relaunches so the user sees the
	// new theme in the list.
	openInstallAfter bool
	// openSettingsAfter tunnels the `s` intent to launch the settings panel.
	openSettingsAfter bool
	// P3.5 live-apply debouncer wires in here.
	pending *debouncer
	// Styles derived from the currently-active theme's palette. Rebuilt
	// after every theme swap so the picker itself follows the theme.
	styles pickerStyles
}

// pickerStyles holds all lipgloss styles the TUI uses. Rebuilt via
// reloadStyles() whenever the active theme changes.
type pickerStyles struct {
	BoxTitle     lipgloss.Style
	ListBox      lipgloss.Style
	PreviewBox   lipgloss.Style
	ListItem     lipgloss.Style
	SelectedItem lipgloss.Style
	Help         lipgloss.Style
}

// reloadStyles reads the active theme's palette and rebuilds all styles
// so the TUI paints itself in the theme's colors. Called once at Init
// and after every live-apply swap.
//
// Uses lipgloss.CompleteAdaptiveColor for graceful degradation:
//   - truecolor terminals get the exact palette hex
//   - 256-color terminals get a nearest-color approximation (lipgloss
//     does the reduction internally when the terminal supports fewer colors)
//   - light-terminal fallbacks flip to sensible defaults so text stays
//     readable if palette.toml is unavailable
func (m *pickerModel) reloadStyles() {
	p := LoadPaletteColors(CurrentThemeDir())

	// AdaptiveColor triplets: Light = short-form for light terminals,
	// Dark = short-form for dark terminals. lipgloss picks based on
	// terminal bg detection. We supply the palette color for both since
	// the palette is already computed for the active theme; the second
	// slot is a defensive fallback if lipgloss can't detect the mode.
	accent := lipgloss.AdaptiveColor{Light: p.Accent, Dark: p.Accent}
	bright := lipgloss.AdaptiveColor{Light: p.Bright, Dark: p.Bright}
	muted := lipgloss.AdaptiveColor{Light: p.Muted, Dark: p.Muted}
	selectedBg := lipgloss.AdaptiveColor{Light: p.SelectedBg, Dark: p.SelectedBg}
	fg := lipgloss.AdaptiveColor{Light: p.Fg, Dark: p.Fg}

	m.styles = pickerStyles{
		BoxTitle: lipgloss.NewStyle().Bold(true).Foreground(bright),
		ListBox: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1).
			Width(30),
		PreviewBox: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1).
			Width(50),
		ListItem: lipgloss.NewStyle().Foreground(fg),
		SelectedItem: lipgloss.NewStyle().
			Foreground(bright).
			Background(selectedBg).
			Bold(true),
		Help: lipgloss.NewStyle().Foreground(muted),
	}
}

func (m *pickerModel) Init() tea.Cmd {
	// Kick off first live-apply if enabled and we're on a different theme
	// than the picker opened with.
	return nil
}

func (m *pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		// Reserved: dynamic sizing later.
		return m, nil
	case liveApplyMsg:
		return m.doLiveApply(msg.name)
	}
	return m, nil
}

func (m *pickerModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		// Quit without changes; revert to startedAt if state has drifted
		// from the pre-TUI theme for any reason (live-preview, live-apply
		// toggled off after preview, drift, etc.). Always compare current
		// state to startedAt — NOT the cursor position, which can differ
		// from state after scroll-around.
		m.revertIfNeeded()
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.revertIfNeeded()
		m.quitting = true
		return m, tea.Quit
	case "enter":
		// Commit.
		if err := Set(m.themes[m.cursor].Name, SetOptions{Commit: true}); err != nil {
			m.err = err
		}
		m.quitting = true
		return m, tea.Quit
	case "L":
		m.liveApply = !m.liveApply
		return m, nil
	case "w":
		// Open the wallpaper subpicker for the currently-active theme (the
		// one whose colors are on screen), not the one under cursor. Tunnel
		// the intent via openWallpaperAfter — tea.Sequence(tea.Quit, cmd)
		// doesn't work because Quit terminates the loop before cmd runs.
		m.openWallpaperAfter = true
		m.quitting = true
		return m, tea.Quit
	case "i":
		// Open the install-from-URL prompt. Same tunnel pattern as `w`.
		m.openInstallAfter = true
		m.quitting = true
		return m, tea.Quit
	case "s":
		// Open the settings panel for the active theme. Same tunnel pattern.
		m.openSettingsAfter = true
		m.quitting = true
		return m, tea.Quit
	case "W":
		// Cycle to the next wallpaper for the active theme without
		// opening the picker. Fast repeat-friendly.
		if s := activeState(); s.Theme != "" {
			_ = CycleWallpaper(s.Theme)
		}
		return m, nil
	case "up", "k":
		return m.move(-1)
	case "down", "j":
		return m.move(1)
	case "home", "g":
		m.cursor = 0
		return m.maybeSchedule()
	case "end", "G":
		m.cursor = len(m.themes) - 1
		return m.maybeSchedule()
	}
	return m, nil
}

// revertIfNeeded restores the pre-TUI theme when live-preview has left
// state pointing at anything other than startedAt. Compares CURRENT
// STATE (not cursor position) so scrolling around then landing back
// on startedAt still triggers the revert if the debouncer fired for an
// intermediate theme.
func (m *pickerModel) revertIfNeeded() {
	if m.startedAt == "" {
		return
	}
	s := activeState()
	if s.Theme == "" || s.Theme == m.startedAt {
		return
	}
	// Cancel any pending debounced live-apply so it doesn't re-set
	// state to the previewed theme AFTER we revert.
	if m.pending != nil {
		m.pending.cancel()
	}
	_ = Set(m.startedAt, SetOptions{Commit: false})
}

func (m *pickerModel) move(delta int) (tea.Model, tea.Cmd) {	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.themes) {
		m.cursor = len(m.themes) - 1
	}
	return m.maybeSchedule()
}

func (m *pickerModel) View() string {
	if m.quitting {
		return ""
	}
	left := m.renderList()
	right := m.renderPreview()
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	help := m.styles.Help.Render(
		"↑↓ navigate · ⏎ commit · esc revert · L toggle-live (" + liveLabel(m.liveApply) + ") · w wallpaper · W cycle-wp · s settings · i install · q quit",
	)
	return lipgloss.JoinVertical(lipgloss.Left, body, "", help)
}

func liveLabel(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func (m *pickerModel) renderList() string {
	var lines []string
	title := m.styles.BoxTitle.Render(fmt.Sprintf("Themes (%d)", len(m.themes)))
	lines = append(lines, title)
	for i, t := range m.themes {
		prefix := "  "
		style := m.styles.ListItem
		if i == m.cursor {
			prefix = "▸ "
			style = m.styles.SelectedItem
		}
		mark := " "
		if t.Current {
			mark = "*"
		}
		row := fmt.Sprintf("%s%s %s", prefix, mark, t.Name)
		lines = append(lines, style.Render(row))
	}
	return m.styles.ListBox.Render(strings.Join(lines, "\n"))
}

func (m *pickerModel) renderPreview() string {
	if m.cursor < 0 || m.cursor >= len(m.themes) {
		return m.styles.PreviewBox.Render("(no selection)")
	}
	t := m.themes[m.cursor]
	swatches := renderSwatches(t)
	var meta []string
	meta = append(meta, m.styles.BoxTitle.Render("Preview"))
	meta = append(meta, "")
	meta = append(meta, swatches)
	meta = append(meta, "")
	if credits := loadShort(filepath.Join(t.Path, "CREDITS.md")); credits != "" {
		meta = append(meta, credits)
	}
	meta = append(meta, fmt.Sprintf("Wallpapers: %d", t.WallpaperCount))
	if t.HasPalette {
		meta = append(meta, "Enriched palette: yes")
	} else {
		meta = append(meta, "Enriched palette: no (ANSI-derived fallbacks)")
	}
	return m.styles.PreviewBox.Render(strings.Join(meta, "\n"))
}

// renderSwatches reads the theme's theme.json ANSI palette and draws
// colored blocks. Falls back to '(no swatches)' when theme.json is
// missing or unparseable.
func renderSwatches(t ThemeInfo) string {
	th, err := palette.Load(t.Path)
	if err != nil {
		return "(no swatches)"
	}
	block := "██"
	var line1, line2 []string
	for i := 0; i < 8; i++ {
		line1 = append(line1, lipgloss.NewStyle().Foreground(lipgloss.Color(th.Palette.Ansi[i])).Render(block))
	}
	for i := 8; i < 16; i++ {
		line2 = append(line2, lipgloss.NewStyle().Foreground(lipgloss.Color(th.Palette.Ansi[i])).Render(block))
	}
	return strings.Join(line1, " ") + "\n" + strings.Join(line2, " ")
}

func loadShort(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// Return the first "## Upstream" section or the first two non-empty lines.
	lines := strings.Split(string(b), "\n")
	var out []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, l)
		if len(out) >= 2 {
			break
		}
	}
	return strings.Join(out, "\n")
}

// -- styles ---------------------------------------------------------------
// Styles are now built per-model via reloadStyles(); the previous package-level
// vars were removed so the TUI always follows the currently-active theme.
