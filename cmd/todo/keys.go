package main

import "github.com/charmbracelet/bubbletea"

// Key constants for readability.
const (
	KeyQ        = "q"
	KeyJ        = "j"
	KeyK        = "k"
	KeyG        = "g"
	KeyD        = "d"
	KeyA        = "a"
	KeyE        = "e"
	KeyS        = "s"
	KeySlash    = "/"
	KeyTab      = "tab"
	KeyShiftTab = "shift+tab"
	KeyEnter    = "enter"
	KeyEsc      = "esc"
	KeyCtrlC    = "ctrl+c"
)

// Help returns the context-sensitive help bar text.
func Help(mode ViewMode) string {
	switch mode {
	case ModeFilter:
		return StyleHelpKey.Render("esc") + StyleHelp.Render(" clear  ") +
			StyleHelpKey.Render("enter") + StyleHelp.Render(" apply")
	case ModeForm:
		return StyleHelpKey.Render("j/k") + StyleHelp.Render(" nav  ") +
			StyleHelpKey.Render("enter") + StyleHelp.Render(" edit/cycle  ") +
			StyleHelpKey.Render("tab") + StyleHelp.Render(" actions  ") +
			StyleHelpKey.Render("esc") + StyleHelp.Render(" cancel")
	case ModeConfirm:
		return StyleHelpKey.Render("y") + StyleHelp.Render(" confirm  ") +
			StyleHelpKey.Render("n/esc") + StyleHelp.Render(" cancel")
	default:
		return StyleHelpKey.Render("j/k") + StyleHelp.Render(" nav  ") +
			StyleHelpKey.Render("enter") + StyleHelp.Render(" done  ") +
			StyleHelpKey.Render("a") + StyleHelp.Render(" add  ") +
			StyleHelpKey.Render("e") + StyleHelp.Render(" edit  ") +
			StyleHelpKey.Render("d") + StyleHelp.Render(" del  ") +
			StyleHelpKey.Render("/") + StyleHelp.Render(" filter  ") +
			StyleHelpKey.Render("tab") + StyleHelp.Render(" tabs  ") +
			StyleHelpKey.Render("q") + StyleHelp.Render(" quit")
	}
}

// ViewMode tracks what the TUI is currently showing.
type ViewMode int

const (
	ModeList ViewMode = iota
	ModeFilter
	ModeForm
	ModeConfirm
)

// Msg types for internal communication.

// TasksUpdatedMsg signals that the task list needs refreshing.
type TasksUpdatedMsg struct{}

// SwitchTabMsg signals a tab switch.
type SwitchTabMsg struct{ Index int }

// OpenFormMsg signals the form should open.
type OpenFormMsg struct {
	EditTask *int // nil = add new, non-nil = edit existing ID
}

// CloseFormMsg signals the form closed.
type CloseFormMsg struct {
	Canceled bool
}

func taskUpdatedCmd() tea.Cmd {
	return func() tea.Msg { return TasksUpdatedMsg{} }
}
