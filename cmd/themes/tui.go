package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
	xdgstate "github.com/PedroKlein/tools/cmd/themes/internal/state"
)

const (
	wallpaperPreviewDelay    = 200 * time.Millisecond
	wallpaperPreviewTimeout  = 2 * time.Second
	sketchybarPreviewDelay   = 200 * time.Millisecond
	sketchybarPreviewTimeout = 2 * time.Second
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
// Scroll preview applies fast color hooks immediately and debounces wallpaper.
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

type wallpaperPreviewMsg struct {
	seq   int
	theme string
}

type wallpaperPreviewDoneMsg struct {
	seq int
	err error
}

type sketchybarPreviewMsg struct {
	seq   int
	theme string
}

type sketchybarPreviewDoneMsg struct {
	seq int
	err error
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
	// Styles derived from the currently-active theme's palette. Rebuilt
	// after every preview swap so the picker itself follows the theme.
	styles pickerStyles
	// preview sequence counters invalidate stale debounce messages and stale
	// setter completions when the cursor moves again or the picker exits.
	wallpaperPreviewSeq  int
	sketchybarPreviewSeq int
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
	case wallpaperPreviewMsg:
		return m.handleWallpaperPreview(msg)
	case wallpaperPreviewDoneMsg:
		return m.handleWallpaperPreviewDone(msg)
	case sketchybarPreviewMsg:
		return m.handleSketchybarPreview(msg)
	case sketchybarPreviewDoneMsg:
		return m.handleSketchybarPreviewDone(msg)
	}
	_ = msg
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
		m.cancelPreviews()
		m.revertIfNeeded()
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.cancelPreviews()
		m.revertIfNeeded()
		m.quitting = true
		return m, tea.Quit
	case "enter":
		m.cancelPreviews()
		if err := Set(m.themes[m.cursor].Name, SetOptions{Commit: true}); err != nil {
			m.err = err
		}
		m.quitting = true
		return m, tea.Quit
	case "L":
		m.liveApply = !m.liveApply
		m.cancelPreviews()
		return m, nil
	case "w":
		// Open the wallpaper subpicker for the currently-active theme (the
		// one whose colors are on screen), not the one under cursor. Tunnel
		// the intent via openWallpaperAfter — tea.Sequence(tea.Quit, cmd)
		// doesn't work because Quit terminates the loop before cmd runs.
		m.cancelPreviews()
		m.openWallpaperAfter = true
		m.quitting = true
		return m, tea.Quit
	case "i":
		// Open the install-from-URL prompt. Same tunnel pattern as `w`.
		m.cancelPreviews()
		m.openInstallAfter = true
		m.quitting = true
		return m, tea.Quit
	case "s":
		// Open the settings panel for the active theme. Same tunnel pattern.
		m.cancelPreviews()
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
		return m.applyPreview()
	case "end", "G":
		m.cursor = len(m.themes) - 1
		return m.applyPreview()
	}
	return m, nil
}

// xdgCurrentThemeName returns the basename of the theme the XDG
// `current` symlink points at, or "" on any error. Used by
// revertIfNeeded to check the visible surface state without touching
// state.json (which preview never writes).
func xdgCurrentThemeName() string {
	return xdgstate.CurrentTargetTheme()
}

// revertIfNeeded restores the pre-TUI theme when preview has left the
// symlink pointing at anything other than startedAt. Compares the
// CURRENT SYMLINK TARGET (not state.json, which preview never writes)
// so scrolling around then landing back on startedAt still triggers the
// revert if an intermediate theme's preview retinted the surfaces.
func (m *pickerModel) revertIfNeeded() {
	if m.startedAt == "" {
		return
	}
	// Preview does NOT write state.json, so activeState() is not the
	// right signal. Compare against the current symlink target.
	current := xdgCurrentThemeName()
	if current != "" && current != m.startedAt {
		_ = Set(m.startedAt, SetOptions{Commit: false, SkipHooks: []string{"wallpaper"}})
		_ = PreviewSketchybar(m.startedAt)
	}
	_ = applyWallpaperHook(themeDir(m.startedAt))
}

func (m *pickerModel) move(delta int) (tea.Model, tea.Cmd) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.themes) {
		m.cursor = len(m.themes) - 1
	}
	return m.applyPreview()
}

func (m *pickerModel) cancelPreviews() {
	m.wallpaperPreviewSeq++
	m.sketchybarPreviewSeq++
}

func (m *pickerModel) scheduleWallpaperPreview(theme string) tea.Cmd {
	m.wallpaperPreviewSeq++
	seq := m.wallpaperPreviewSeq
	return tea.Tick(wallpaperPreviewDelay, func(time.Time) tea.Msg {
		return wallpaperPreviewMsg{seq: seq, theme: theme}
	})
}

func (m *pickerModel) handleWallpaperPreview(msg wallpaperPreviewMsg) (tea.Model, tea.Cmd) {
	if msg.seq != m.wallpaperPreviewSeq || !m.liveApply || m.quitting || m.themes[m.cursor].Name != msg.theme {
		return m, nil
	}
	return m, func() tea.Msg {
		return wallpaperPreviewDoneMsg{seq: msg.seq, err: PreviewWallpaper(msg.theme)}
	}
}

func (m *pickerModel) handleWallpaperPreviewDone(msg wallpaperPreviewDoneMsg) (tea.Model, tea.Cmd) {
	// Preview errors are intentionally non-fatal; commit still reports hook
	// failures through the normal Set path.
	_ = msg
	return m, nil
}

func (m *pickerModel) scheduleSketchybarPreview(theme string) tea.Cmd {
	m.sketchybarPreviewSeq++
	seq := m.sketchybarPreviewSeq
	return tea.Tick(sketchybarPreviewDelay, func(time.Time) tea.Msg {
		return sketchybarPreviewMsg{seq: seq, theme: theme}
	})
}

func (m *pickerModel) handleSketchybarPreview(msg sketchybarPreviewMsg) (tea.Model, tea.Cmd) {
	if msg.seq != m.sketchybarPreviewSeq || !m.liveApply || m.quitting || m.themes[m.cursor].Name != msg.theme {
		return m, nil
	}
	return m, func() tea.Msg {
		return sketchybarPreviewDoneMsg{seq: msg.seq, err: PreviewSketchybar(msg.theme)}
	}
}

func (m *pickerModel) handleSketchybarPreviewDone(msg sketchybarPreviewDoneMsg) (tea.Model, tea.Cmd) {
	// Preview errors are intentionally non-fatal; commit still reports hook
	// failures through the normal Set path.
	_ = msg
	return m, nil
}

// applyPreview is called on cursor moves. It keeps fast retint hooks in the
// synchronous Set path, but schedules wallpaper and sketchybar preview after
// cursor idle so slow macOS setters/reloads do not block Bubble Tea key handling.
func (m *pickerModel) applyPreview() (tea.Model, tea.Cmd) {
	if !m.liveApply {
		return m, nil
	}
	name := m.themes[m.cursor].Name
	_ = Set(name, SetOptions{Commit: false, SkipHooks: []string{"wallpaper"}})
	m.reloadStyles()
	for i := range m.themes {
		m.themes[i].Current = m.themes[i].Name == name
	}
	return m, tea.Batch(m.scheduleWallpaperPreview(name), m.scheduleSketchybarPreview(name))
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
