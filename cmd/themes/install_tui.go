package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// runInstallInteractive opens a text-input prompt for a git URL, clones
// the theme, and re-launches the picker with the new entry in focus.
//
// Prompted flow (post-picker-quit dispatch, same tunnel as `w` wallpaper):
//   1. Show "Install from URL:" input at the top of the alt-screen.
//   2. User types URL or paste-injects it. Enter clones + derives.
//   3. On success, re-launch the picker with cursor on the new theme.
//      On failure, print the error and return to the shell.
//
// Uses a minimal handwritten textinput rather than the charm/bubbles
// package to keep the tools repo dep list at Charm-minimum.
func runInstallInteractive() string {
	m := &installPromptModel{}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "themes:", err)
		os.Exit(ExitError)
	}
	if m.cancelled {
		return ""
	}
	if m.err != nil {
		fmt.Fprintln(os.Stderr, "themes install:", m.err)
		return ""
	}
	// Successful install \u2014 re-launch the picker so the user sees the new
	// theme in-list. We can't call runInteractive() directly (would recurse
	// via alt-screen state); the CLI dispatch does that when main returns.
	fmt.Fprintf(os.Stderr, "themes install: %s ready.\n", m.installed)
	return m.installed
}

// installPromptModel is a minimal single-line text input.
type installPromptModel struct {
	input      string
	cursor     int    // insert position within input
	err        error
	installed  string // theme name after successful install
	cancelled  bool
	installing bool
	spinner    int // frame counter for the spinner
}

func (m *installPromptModel) Init() tea.Cmd {
	// Kick a periodic tick during installing state so the spinner animates.
	return nil
}

func (m *installPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		if m.installing {
			// Ignore keys while a clone is in progress.
			return m, nil
		}
		switch km.Type {
		case tea.KeyEscape, tea.KeyCtrlC:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyEnter:
			if strings.TrimSpace(m.input) == "" {
				return m, nil
			}
			// v4: install is superseded by /theme-import (see P4). The
			// TUI pane no longer clones + parses upstream; it displays a
			// pointer to the slash command instead.
			m.installing = false
			return m, func() tea.Msg {
				url := strings.TrimSpace(m.input)
				return installResultMsg{name: "", err: fmt.Errorf(
					"install superseded; run /theme-import %s in Pi", url)}
			}
		case tea.KeyBackspace:
			if m.cursor > 0 {
				m.input = m.input[:m.cursor-1] + m.input[m.cursor:]
				m.cursor--
			}
		case tea.KeyLeft:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyRight:
			if m.cursor < len(m.input) {
				m.cursor++
			}
		case tea.KeyRunes, tea.KeySpace:
			// Insert typed characters at cursor.
			runes := string(km.Runes)
			if km.Type == tea.KeySpace {
				runes = " "
			}
			m.input = m.input[:m.cursor] + runes + m.input[m.cursor:]
			m.cursor += len(runes)
		}
	}
	if r, ok := msg.(installResultMsg); ok {
		m.installing = false
		m.installed = r.name
		m.err = r.err
		return m, tea.Quit
	}
	return m, nil
}

func (m *installPromptModel) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#63B07A")).
		Render("Install from URL")
	prompt := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#549E6A")).
		Render("\u276f ")

	// Simple caret rendering: split input at cursor, print reverse-video
	// character where cursor is.
	var body string
	if m.installing {
		body = "cloning\u2026"
	} else {
		before := m.input[:m.cursor]
		after := ""
		if m.cursor < len(m.input) {
			after = m.input[m.cursor:]
		}
		body = before + "\u2588" + after
	}
	inputLine := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#509475")).
		Padding(0, 1).
		Width(60).
		Render(prompt + body)

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#627A6C")).
		Render("\u23ce install \u00b7 esc cancel")

	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#627A6C")).
		Render("Paste any git URL, e.g. https://github.com/user/omarchy-<name>-theme")

	return lipgloss.JoinVertical(lipgloss.Left, title, "", inputLine, "", hint, "", help)
}

// installResultMsg carries the install outcome back to Update.
type installResultMsg struct {
	name string
	err  error
}
