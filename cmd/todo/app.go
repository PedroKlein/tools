package main

import (
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// App is the top-level bubbletea model composing tabs, list, and form.
type App struct {
	store  *Store
	repoID string

	tabs TabModel
	list ListModel
	form *FormModel // nil when form is not open

	mode   ViewMode
	width  int
	height int

	// confirm delete state
	confirmID    int
	confirmTitle string
}

// NewApp creates the TUI application model.
func NewApp(store *Store, repoID string) App {
	tabs := NewTabModel(store, repoID)
	list := NewListModel(store, tabs.ActiveScope())

	return App{
		store:  store,
		repoID: repoID,
		tabs:   tabs,
		list:   list,
		mode:   ModeList,
	}
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.tabs.width = msg.Width
		a.list.width = msg.Width
		a.list.height = msg.Height - 4 // tabs + help bar

		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)

	case TasksUpdatedMsg:
		a.list.Reload(a.store, a.tabs.ActiveScope())
		a.tabs.UpdateCounts(a.store)

		return a, nil

	case SwitchTabMsg:
		a.tabs.active = msg.Index
		a.list.Reload(a.store, a.tabs.ActiveScope())

		return a, nil
	}

	return a, nil
}

func (a App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	// Tab bar (full width)
	tabBar := a.tabs.View()

	// Help bar (full width, pinned to bottom)
	helpBar := Help(a.mode)
	if a.mode == ModeFilter {
		helpBar = StyleAccent.Render("/") + StyleFg.Render(a.list.filterInput+"█") + "  " + helpBar
	}

	helpBar = StyleHelpBar.Width(a.width).Render(helpBar)

	// Calculate content height (full screen minus tab bar and help bar)
	contentHeight := max(a.height-lipgloss.Height(tabBar)-lipgloss.Height(helpBar), 1)

	// Main content area
	var content string

	switch a.mode {
	case ModeForm:
		if a.form != nil {
			content = lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center,
				a.form.View(a.width, contentHeight))
		}
	case ModeConfirm:
		listView := a.list.View()
		confirmMsg := "\n" + StyleError.Render("  Delete ") +
			StyleTaskSelected.Render("\""+a.confirmTitle+"\"") +
			StyleError.Render("? (y/n)")
		content = listView + confirmMsg
	default:
		content = a.list.View()
	}

	// Ensure content fills the available height
	content = StyleContent.Width(a.width).Height(contentHeight).Render(content)

	// Compose full screen
	screen := lipgloss.JoinVertical(lipgloss.Left, tabBar, content, helpBar)

	// Apply full-screen background
	return StyleApp.Width(a.width).Height(a.height).Render(screen)
}

// Run starts the TUI application.
func Run(store *Store, repoID string) error {
	app := NewApp(store, repoID)
	p := tea.NewProgram(app, tea.WithAltScreen())

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}

func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global quit
	if key == KeyCtrlC {
		return a, tea.Quit
	}

	switch a.mode {
	case ModeForm:
		return a.handleFormKey(msg)
	case ModeConfirm:
		return a.handleConfirmKey(msg)
	case ModeFilter:
		return a.handleFilterKey(msg)
	default:
		return a.handleListKey(msg)
	}
}

func (a App) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case KeyQ:
		return a, tea.Quit

	case KeyJ, "down":
		a.list.CursorDown()
	case KeyK, "up":
		a.list.CursorUp()
	case KeyG:
		a.list.CursorTop()
	case "G":
		a.list.CursorBottom()

	case KeyTab:
		a.tabs.Next()
		a.list.Reload(a.store, a.tabs.ActiveScope())
	case KeyShiftTab:
		a.tabs.Prev()
		a.list.Reload(a.store, a.tabs.ActiveScope())

	case KeyEnter:
		a.toggleSelectedTaskStatus()

	case KeyA:
		a.form = NewFormModel(nil, a.repoID, a.width)
		a.mode = ModeForm

	case KeyE:
		a.openEditForm()

	case KeyD:
		a.openDeleteConfirm()

	case KeySlash:
		a.mode = ModeFilter
		a.list.filterInput = ""

	case KeyS:
		a.list.CycleStatusFilter()
		a.list.ApplyFilters()
	}

	return a, nil
}

func (a App) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.form == nil {
		a.mode = ModeList
		return a, nil
	}

	result := a.form.HandleKey(msg)

	switch result {
	case FormResultSave:
		parsed := a.form.ToParsedTask()

		scope := a.form.Scope()
		if a.form.editID != nil {
			// Update existing task
			a.updateTask(*a.form.editID, parsed)
		} else {
			// Create new task
			a.addTask(parsed, scope)
		}

		a.form = nil
		a.mode = ModeList

		return a, taskUpdatedCmd()

	case FormResultCancel:
		a.form = nil
		a.mode = ModeList

	default:
		// FormResultNone: no action
	}

	return a, nil
}

func (a App) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "y", "Y":
		a.deleteTask(a.confirmID)
		a.mode = ModeList

		return a, taskUpdatedCmd()
	case "n", "N", KeyEsc:
		a.mode = ModeList
	}

	return a, nil
}

func (a App) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case KeyEsc:
		a.list.filterInput = ""
		a.list.ApplyFilters()
		a.mode = ModeList
	case KeyEnter:
		a.list.ApplyFilters()
		a.mode = ModeList
	case "backspace":
		if a.list.filterInput != "" {
			a.list.filterInput = a.list.filterInput[:len(a.list.filterInput)-1]
			a.list.ApplyFilters()
		}
	default:
		if len(key) == 1 && key[0] >= 32 {
			a.list.filterInput += key
			a.list.ApplyFilters()
		}
	}

	return a, nil
}

func (a *App) addTask(parsed ParsedTask, scope string) {
	nextID, _ := a.store.NextID(scope)
	task := NewTask(nextID, parsed.Title, scope, parsed.Type)
	task.Priority = parsed.Priority
	task.DueDate = parsed.DueDate
	task.Description = parsed.Description

	tasks, _ := a.store.LoadScope(scope)
	tasks = append(tasks, task)
	_ = a.store.SaveScope(scope, tasks)
}

func (a *App) updateTask(id int, parsed ParsedTask) {
	scopes, _ := a.store.ListScopes()
	for _, scope := range scopes {
		tasks, _ := a.store.LoadScope(scope)
		for i := range tasks {
			if tasks[i].ID != id {
				continue
			}

			tasks[i].Title = parsed.Title
			tasks[i].Type = parsed.Type
			tasks[i].Priority = parsed.Priority
			tasks[i].DueDate = parsed.DueDate
			tasks[i].Description = parsed.Description
			_ = a.store.SaveScope(scope, tasks)

			return
		}
	}
}

func (a *App) deleteTask(id int) {
	scopes, _ := a.store.ListScopes()
	for _, scope := range scopes {
		tasks, _ := a.store.LoadScope(scope)
		for i := range tasks {
			if tasks[i].ID != id {
				continue
			}

			tasks = append(tasks[:i], tasks[i+1:]...)
			_ = a.store.SaveScope(scope, tasks)

			return
		}
	}
}

func (a *App) toggleSelectedTaskStatus() {
	task := a.list.SelectedTask()
	if task == nil {
		return
	}

	if task.Status == StatusDone {
		task.Status = StatusOpen
	} else {
		task.Status = StatusDone
	}

	_ = a.store.Save(a.list.tasks)
	a.list.Reload(a.store, a.tabs.ActiveScope())
	a.tabs.UpdateCounts(a.store)
}

func (a *App) openEditForm() {
	task := a.list.SelectedTask()
	if task == nil {
		return
	}

	a.form = NewFormModel(task, a.repoID, a.width)
	a.mode = ModeForm
}

func (a *App) openDeleteConfirm() {
	task := a.list.SelectedTask()
	if task == nil {
		return
	}

	a.confirmID = task.ID
	a.confirmTitle = task.Title
	a.mode = ModeConfirm
}
