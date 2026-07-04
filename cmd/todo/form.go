package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FormResult represents the outcome of form interaction.
type FormResult int

const (
	FormResultNone FormResult = iota
	FormResultSave
	FormResultCancel
)

// FormModel handles task add/edit.
type FormModel struct {
	fields      []formField
	fieldIndex  int
	focusAction bool // true when focus is on action buttons
	actionIndex int  // 0=save, 1=discard
	editing     bool
	editBuffer  string
	editID      *int // nil for new, non-nil for editing existing

	width int
}

type formField struct {
	label  string
	value  string
	cycler []string // nil means text input, non-nil means cycle through options
}

// NewFormModel creates a form, optionally pre-filled from an existing task.
func NewFormModel(task *Task, repoID string, width int) *FormModel {
	types := make([]string, len(ValidTypes))
	for i, t := range ValidTypes {
		types[i] = string(t)
	}

	priorities := make([]string, len(ValidPriorities))
	for i, p := range ValidPriorities {
		priorities[i] = string(p)
	}

	// Scope options: current repo, global, work
	scopeOptions := []string{repoID, GlobalRepoID, WorkRepoID}
	// Deduplicate if repoID is already global or work
	seen := make(map[string]bool)

	var scopes []string

	for _, s := range scopeOptions {
		if !seen[s] {
			scopes = append(scopes, s)
			seen[s] = true
		}
	}

	f := &FormModel{
		fields: []formField{
			{label: "Title", value: "", cycler: nil},
			{label: "Type", value: "chore", cycler: types},
			{label: "Priority", value: "medium", cycler: priorities},
			{label: "Scope", value: repoID, cycler: scopes},
			{label: "Due Date", value: "", cycler: nil},
			{label: "Description", value: "", cycler: nil},
		},
		width: width,
	}

	if task != nil {
		f.editID = &task.ID
		f.fields[0].value = task.Title
		f.fields[1].value = string(task.Type)
		f.fields[2].value = string(task.Priority)
		f.fields[3].value = task.RepoID
		f.fields[4].value = task.DueDate
		f.fields[5].value = task.Description
	}

	return f
}

// HandleKey processes a keypress and returns the result.
func (f *FormModel) HandleKey(msg tea.KeyMsg) FormResult {
	key := msg.String()

	if f.editing {
		return f.handleEditKey(key)
	}

	return f.handleNavKey(key)
}

// ToParsedTask converts form fields to a ParsedTask.
func (f *FormModel) ToParsedTask() ParsedTask {
	return ParsedTask{
		Title:       strings.TrimSpace(f.fields[0].value),
		Type:        TaskType(f.fields[1].value),
		Priority:    TaskPriority(f.fields[2].value),
		DueDate:     f.fields[4].value,
		Description: f.fields[5].value,
	}
}

// Scope returns the selected scope from the form.
func (f *FormModel) Scope() string {
	return f.fields[3].value
}

// View renders the form.
func (f *FormModel) View(width, _ int) string {
	formWidth := min(width-4, 70)

	lines := make([]string, 0, len(f.fields)+6)
	lines = append(lines, "")

	title := "📋 New Task"
	if f.editID != nil {
		title = fmt.Sprintf("📋 Edit Task #%d", *f.editID)
	}

	lines = append(lines, StyleTitle.Render("  "+title), "")

	for i, field := range f.fields {
		lines = append(lines, f.renderField(i, field))
	}

	// Action buttons
	actionLine := "    "
	saveLabel := "[ Save ]"
	discardLabel := "[ Discard ]"

	if f.focusAction && f.actionIndex == 0 {
		actionLine += StyleButton.Render(saveLabel) + "  " + StyleButtonInactive.Render(discardLabel)
	} else if f.focusAction && f.actionIndex == 1 {
		actionLine += StyleButtonInactive.Render(saveLabel) + "  " + StyleButton.Render(discardLabel)
	} else {
		actionLine += StyleButtonInactive.Render(saveLabel) + "  " + StyleButtonInactive.Render(discardLabel)
	}

	lines = append(lines, "", actionLine, "")

	content := strings.Join(lines, "\n")

	return StyleFormBorder.Width(formWidth).Render(content)
}

// renderField renders a single form field row.
func (f *FormModel) renderField(i int, field formField) string {
	selected := !f.focusAction && i == f.fieldIndex

	prefix := "  "
	if selected {
		prefix = StyleAccent.Render("▸ ")
	}

	label := StyleFormLabel.Render(field.label + ":")

	var value string

	if f.editing && selected {
		value = StyleFormActive.Render(f.editBuffer + "█")
	} else if field.value == "" {
		value = StyleDim.Render("(empty)")
	} else if field.label == "Scope" {
		value = StyleFormValue.Render(shortScopeName(field.value))
	} else {
		value = StyleFormValue.Render(field.value)
	}

	hint := ""

	if selected && !f.editing {
		if field.cycler != nil {
			hint = StyleDim.Render(" ↵ cycle")
		} else {
			hint = StyleDim.Render(" ↵ edit")
		}
	}

	return prefix + label + value + hint
}

func (f *FormModel) handleNavKey(key string) FormResult {
	switch key {
	case KeyEsc:
		return FormResultCancel

	case KeyJ, "down":
		f.moveFocusDown()

	case KeyK, "up":
		f.moveFocusUp()

	case "h", "left":
		if f.focusAction && f.actionIndex > 0 {
			f.actionIndex--
		}

	case "l", "right":
		if f.focusAction && f.actionIndex < 1 {
			f.actionIndex++
		}

	case KeyTab:
		f.toggleFocusArea()

	case KeyEnter:
		return f.handleEnterKey()
	}

	return FormResultNone
}

func (f *FormModel) moveFocusDown() {
	if f.focusAction {
		return
	}

	if f.fieldIndex < len(f.fields)-1 {
		f.fieldIndex++
	} else {
		f.focusAction = true
		f.actionIndex = 0
	}
}

func (f *FormModel) moveFocusUp() {
	if f.focusAction {
		f.focusAction = false
		f.fieldIndex = len(f.fields) - 1

		return
	}

	if f.fieldIndex > 0 {
		f.fieldIndex--
	}
}

func (f *FormModel) toggleFocusArea() {
	if f.focusAction {
		f.focusAction = false
		f.fieldIndex = 0
	} else {
		f.focusAction = true
		f.actionIndex = 0
	}
}

func (f *FormModel) handleEnterKey() FormResult {
	if f.focusAction {
		if f.actionIndex == 0 {
			if strings.TrimSpace(f.fields[0].value) == "" {
				return FormResultNone
			}

			return FormResultSave
		}

		return FormResultCancel
	}

	field := &f.fields[f.fieldIndex]
	if field.cycler != nil {
		cycled := false
		for i, v := range field.cycler {
			if v == field.value {
				field.value = field.cycler[(i+1)%len(field.cycler)]
				cycled = true

				break
			}
		}

		if !cycled && len(field.cycler) > 0 {
			field.value = field.cycler[0]
		}
	} else {
		f.editing = true
		f.editBuffer = field.value
	}

	return FormResultNone
}

func (f *FormModel) handleEditKey(key string) FormResult {
	switch key {
	case KeyEnter:
		f.fields[f.fieldIndex].value = strings.TrimSpace(f.editBuffer)
		f.editing = false
	case KeyEsc:
		f.editing = false
	case "backspace":
		if f.editBuffer != "" {
			f.editBuffer = f.editBuffer[:len(f.editBuffer)-1]
		}
	default:
		if len(key) == 1 && key[0] >= 32 {
			f.editBuffer += key
		}
	}

	return FormResultNone
}

// Place centers content in the available space using lipgloss.
func Place(content string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}
