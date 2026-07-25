package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// runWallpaperInteractive is the TUI-driven wallpaper picker.
//
// It lists the current theme's backgrounds/ files, shows a text-panel
// preview (chafa rendering when available; filename+size otherwise), and
// on Enter sets that wallpaper via SetWallpaper. Esc reverts.
//
// Chafa is preferred because it renders inline in any true-color terminal.
// Kitty-graphics (icat) works only in Ghostty/kitty. If neither is present,
// the picker still functions as a scrollable filename list.
func runWallpaperInteractive() {
	s := activeState()
	if s.Theme == "" {
		dieMsg("no theme active; run: themes set <name>", ExitNotFound)
	}
	list, err := WallpaperList(s.Theme)
	if err != nil {
		dieMsg(err.Error(), ExitError)
	}
	if len(list) == 0 {
		dieMsg("no wallpapers for "+s.Theme, ExitNotFound)
	}

	// Sort so state.wallpaper_by_theme's entry is highlighted first.
	initial := 0
	prev := s.WallpaperByTheme[s.Theme]
	if prev == "" {
		prev = s.Wallpaper
	}
	for i, p := range list {
		if p == prev {
			initial = i
			break
		}
	}

	m := &wallpaperPickerModel{
		theme:  s.Theme,
		items:  list,
		cursor: initial,
		prev:   prev,
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

type wallpaperPickerModel struct {
	theme    string
	items    []string
	cursor   int
	prev     string
	err      error
	quitting bool
}

func (m *wallpaperPickerModel) Init() tea.Cmd { return nil }

func (m *wallpaperPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			// Revert to prior wallpaper if we changed anything.
			if m.prev != "" && m.prev != m.items[m.cursor] {
				_ = SetWallpaper(m.prev)
			}
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if err := SetWallpaper(m.items[m.cursor]); err != nil {
				m.err = err
			}
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m *wallpaperPickerModel) View() string {
	if m.quitting {
		return ""
	}
	title := wallpaperTitleStyle.Render("Wallpapers — " + m.theme)
	var lines []string
	for i, p := range m.items {
		style := wallpaperItemStyle
		prefix := "  "
		if i == m.cursor {
			style = wallpaperSelectedStyle
			prefix = "▸ "
		}
		lines = append(lines, style.Render(prefix+filepath.Base(p)))
	}
	left := wallpaperListBoxStyle.Render(strings.Join(lines, "\n"))
	preview := renderWallpaperPreview(m.items[m.cursor])
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, preview)
	help := wallpaperHelpStyle.Render(
		"↑↓ navigate · ⏎ set · esc revert · q quit")
	return lipgloss.JoinVertical(lipgloss.Left, title, body, "", help)
}

func renderWallpaperPreview(path string) string {
	info := fmt.Sprintf("Path: %s\nSize: %s", path, humanSize(path))
	// Try chafa for inline color preview.
	if _, err := exec.LookPath("chafa"); err == nil {
		out, err := exec.Command("chafa", "--size=40x14", "--format=symbols", path).Output()
		if err == nil {
			info += "\n\n" + string(out)
		}
	}
	return wallpaperPreviewBoxStyle.Render(info)
}

func humanSize(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return "?"
	}
	n := fi.Size()
	switch {
	case n > 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	case n > 1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

var (
	wallpaperTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#63B07A")).
				MarginBottom(1)
	wallpaperListBoxStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#509475")).
				Padding(0, 1).
				Width(40)
	wallpaperPreviewBoxStyle = lipgloss.NewStyle().
					BorderStyle(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("#509475")).
					Padding(0, 1).
					Width(50)
	wallpaperItemStyle     = lipgloss.NewStyle()
	wallpaperSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#63B07A")).
				Bold(true)
	wallpaperHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#627A6C"))
)
