package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AddApp is a minimal TUI for reviewing/editing a single task before saving.
type AddApp struct {
	store  *Store
	repoID string
	form   *FormModel
	width  int
	height int
	saved  bool
	aiUsed bool
}

// RunAdd opens a TUI form pre-filled with AI-parsed (or heuristic) data for review.
// If text is empty, opens a blank form.
func RunAdd(store *Store, repoID, text string) error {
	var parsed ParsedTask
	var aiUsed bool
	if text != "" {
		parsed, aiUsed = AIParseWithStatus(text)
	}

	// Build a pseudo-task to seed the form
	seed := &Task{
		Title:       parsed.Title,
		Type:        parsed.Type,
		Priority:    parsed.Priority,
		DueDate:     parsed.DueDate,
		Description: parsed.Description,
		RepoID:      repoID,
	}
	// If empty input, set defaults
	if text == "" {
		seed = nil
	}

	app := &AddApp{
		store:  store,
		repoID: repoID,
		form:   NewFormModel(seed, repoID, 80),
		aiUsed: aiUsed,
	}
	// Clear the editID since this is a new task (not editing existing)
	app.form.editID = nil

	p := tea.NewProgram(app, tea.WithAltScreen())

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("running form: %w", err)
	}

	return nil
}

func (a *AddApp) Init() tea.Cmd {
	return nil
}

func (a *AddApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.form.width = msg.Width

		return a, nil

	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}

		result := a.form.HandleKey(msg)
		switch result {
		case FormResultSave:
			a.saveTask()
			a.saved = true

			return a, tea.Quit
		case FormResultCancel:
			return a, tea.Quit
		case FormResultNone:
			// no action
		}

		return a, nil
	}

	return a, nil
}

func (a *AddApp) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	// Header
	header := StyleTitle.Render("  Add Task")
	if a.form.fields[0].value != "" {
		if a.aiUsed {
			header += StyleDim.Render("  (AI pre-filled, review & save)")
		} else {
			header += StyleDim.Render("  (heuristic pre-fill, q daemon unavailable)")
		}
	}

	// Form centered
	formView := a.form.View(a.width, a.height-4)
	content := lipgloss.Place(a.width, a.height-3, lipgloss.Center, lipgloss.Center, formView)

	// Help
	help := StyleHelpBar.Width(a.width).Render(
		StyleHelpKey.Render("j/k") + StyleHelp.Render(" navigate  ") +
			StyleHelpKey.Render("↵") + StyleHelp.Render(" edit/cycle  ") +
			StyleHelpKey.Render("tab") + StyleHelp.Render(" buttons  ") +
			StyleHelpKey.Render("esc") + StyleHelp.Render(" cancel"))

	sections := []string{header, content, help}
	screen := strings.Join(sections, "\n")

	return StyleApp.Width(a.width).Height(a.height).Render(screen)
}

func (a *AddApp) saveTask() {
	parsed := a.form.ToParsedTask()
	scope := a.form.Scope()

	nextID, _ := a.store.NextID(scope)
	task := NewTask(nextID, parsed.Title, scope, parsed.Type)
	task.Priority = parsed.Priority
	task.DueDate = parsed.DueDate
	task.Description = parsed.Description

	tasks, _ := a.store.LoadScope(scope)
	tasks = append(tasks, task)
	_ = a.store.SaveScope(scope, tasks)
}
