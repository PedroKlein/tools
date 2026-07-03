package main

import (
	"fmt"
	"strings"
)

// ListModel displays and navigates tasks.
type ListModel struct {
	tasks    []Task // all tasks for current scope
	filtered []Task // filtered view
	cursor   int
	width    int
	height   int

	statusFilter StatusFilter
	filterInput  string
}

// NewListModel creates a list model for the given scope.
func NewListModel(store *Store, scope string) ListModel {
	all, _ := store.LoadAll()
	tasks := GetTasksForRepo(all, scope)
	active := FilterByStatus(tasks, FilterActive)
	SortByUrgency(active)

	return ListModel{
		tasks:        tasks,
		filtered:     active,
		cursor:       0,
		statusFilter: FilterActive,
	}
}

// Reload refreshes the task list from the store.
func (l *ListModel) Reload(store *Store, scope string) {
	all, _ := store.LoadAll()
	l.tasks = GetTasksForRepo(all, scope)
	l.ApplyFilters()
}

// ApplyFilters recalculates the filtered view.
func (l *ListModel) ApplyFilters() {
	result := FilterByStatus(l.tasks, l.statusFilter)
	if l.filterInput != "" {
		result = FilterByTitle(result, l.filterInput)
	}

	SortByUrgency(result)
	l.filtered = result

	if l.cursor >= len(l.filtered) {
		l.cursor = max(0, len(l.filtered)-1)
	}
}

// CycleStatusFilter cycles through status filters.
func (l *ListModel) CycleStatusFilter() {
	filters := []StatusFilter{FilterActive, FilterAll, FilterOpen, FilterBlocked, FilterDone}
	for i, f := range filters {
		if f == l.statusFilter {
			l.statusFilter = filters[(i+1)%len(filters)]
			return
		}
	}

	l.statusFilter = FilterActive
}

// CursorDown moves cursor down.
func (l *ListModel) CursorDown() {
	if l.cursor < len(l.filtered)-1 {
		l.cursor++
	}
}

// CursorUp moves cursor up.
func (l *ListModel) CursorUp() {
	if l.cursor > 0 {
		l.cursor--
	}
}

// CursorTop moves cursor to top.
func (l *ListModel) CursorTop() {
	l.cursor = 0
}

// CursorBottom moves cursor to bottom.
func (l *ListModel) CursorBottom() {
	if len(l.filtered) > 0 {
		l.cursor = len(l.filtered) - 1
	}
}

// SelectedTask returns the task at cursor, or nil if empty.
func (l *ListModel) SelectedTask() *Task {
	if l.cursor >= 0 && l.cursor < len(l.filtered) {
		return &l.filtered[l.cursor]
	}

	return nil
}

// View renders the task list.
func (l ListModel) View() string {
	if len(l.filtered) == 0 {
		return StyleDim.Render("\n  No tasks matching current filters.\n")
	}

	var lines []string

	visibleStart, visibleEnd := l.visibleRange()

	// Status filter indicator
	filterLine := StyleMuted.Render(fmt.Sprintf("  [%s]", l.statusFilter))
	if l.filterInput != "" {
		filterLine += StyleDim.Render(fmt.Sprintf(" filter: %q", l.filterInput))
	}

	filterLine += StyleDim.Render(fmt.Sprintf("  %d/%d tasks", len(l.filtered), len(l.tasks)))
	lines = append(lines, filterLine, "")

	for i := visibleStart; i < visibleEnd; i++ {
		task := l.filtered[i]
		selected := i == l.cursor

		line := l.renderTask(task, selected)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (l ListModel) visibleRange() (start, end int) {
	available := l.height - 3 // header + padding
	if available <= 0 {
		available = 20
	}

	start = 0
	if l.cursor >= available {
		start = l.cursor - available + 1
	}

	end = min(end, len(l.filtered))

	return start, end
}

func (l ListModel) renderTask(task Task, selected bool) string {
	// Cursor indicator
	cursor := "  "
	if selected {
		cursor = StyleAccent.Render("▸ ")
	}

	// Status icon
	status := tuiStatusIcon(task.Status)

	// Type badge
	typeBadge := renderTypeBadge(task.Type)

	// Priority
	priority := renderPriority(task.Priority)

	// Title
	var title string

	switch {
	case task.Status == StatusDone:
		title = StyleTaskDone.Render(task.Title)
	case task.Status == StatusBlocked:
		title = StyleTaskBlocked.Render(task.Title)
	case selected:
		title = StyleTaskSelected.Render(task.Title)
	default:
		title = StyleTaskNormal.Render(task.Title)
	}

	// Due date
	due := ""

	if task.DueDate != "" {
		urgency := GetUrgency(task)
		switch urgency {
		case UrgencyOverdue:
			due = " " + StyleError.Render("⚠ "+task.DueDate)
		case UrgencyDueSoon:
			due = " " + StylePriorityMedium.Render("◷ "+task.DueDate)
		default:
			due = " " + StyleDim.Render("◷ "+task.DueDate)
		}
	}

	// ID
	id := StyleDim.Render(fmt.Sprintf("#%-3d", task.ID))

	return fmt.Sprintf("%s%s %s %s %s %s%s", cursor, status, id, typeBadge, priority, title, due)
}

func tuiStatusIcon(s TaskStatus) string {
	switch s {
	case StatusOpen:
		return StyleAccent.Render("○")
	case StatusBlocked:
		return StylePriorityMedium.Render("◌")
	case StatusDone:
		return StyleSuccess.Render("●")
	default:
		return "?"
	}
}

func renderTypeBadge(t TaskType) string {
	label := fmt.Sprintf("%-8s", t)
	switch t {
	case TypeFeature:
		return StyleTypeFeature.Render(label)
	case TypeBug:
		return StyleTypeBug.Render(label)
	case TypeChore:
		return StyleTypeChore.Render(label)
	case TypeResearch:
		return StyleTypeResearch.Render(label)
	case TypeReview:
		return StyleTypeReview.Render(label)
	case TypePersonal:
		return StyleTypePersonal.Render(label)
	default:
		return StyleDim.Render(label)
	}
}

func renderPriority(p TaskPriority) string {
	switch p {
	case PriorityHigh:
		return StylePriorityHigh.Render("★★★")
	case PriorityMedium:
		return StylePriorityMedium.Render("★★ ")
	case PriorityLow:
		return StylePriorityLow.Render("★  ")
	default:
		return "   "
	}
}
