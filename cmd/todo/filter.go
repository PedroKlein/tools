package main

import (
	"sort"
	"strings"
	"time"
)

// Urgency represents how urgent a task is (for sort ordering and display).
type Urgency int

const (
	UrgencyOverdue Urgency = iota // past due date
	UrgencyDueSoon                // within 48 hours
	UrgencyBlocked                // status = blocked
	UrgencyNormal                 // open, no immediate deadline
	UrgencyDone                   // completed
)

// GetUrgency computes the urgency level for a task.
func GetUrgency(task Task) Urgency {
	if task.Status == StatusDone {
		return UrgencyDone
	}

	if task.Status == StatusBlocked {
		return UrgencyBlocked
	}

	return checkDueDateUrgency(task.DueDate)
}

// checkDueDateUrgency computes urgency based on due date alone.
func checkDueDateUrgency(dueDate string) Urgency {
	if dueDate == "" {
		return UrgencyNormal
	}

	due, err := time.Parse("2006-01-02", dueDate)
	if err != nil {
		return UrgencyNormal
	}

	now := time.Now()
	dueEnd := due.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	if dueEnd.Before(now) {
		return UrgencyOverdue
	}

	if dueEnd.Sub(now) < 48*time.Hour {
		return UrgencyDueSoon
	}

	return UrgencyNormal
}

// SortByUrgency sorts tasks: overdue > due-soon > blocked > normal > done,
// then by priority desc, then by creation date asc.
func SortByUrgency(tasks []Task) {
	priorityOrder := map[TaskPriority]int{
		PriorityHigh:   0,
		PriorityMedium: 1,
		PriorityLow:    2,
	}

	sort.SliceStable(tasks, func(i, j int) bool {
		ui := int(GetUrgency(tasks[i]))

		uj := int(GetUrgency(tasks[j]))
		if ui != uj {
			return ui < uj
		}

		pi := priorityOrder[tasks[i].Priority]

		pj := priorityOrder[tasks[j].Priority]
		if pi != pj {
			return pi < pj
		}

		return tasks[i].CreatedAt < tasks[j].CreatedAt
	})
}

// StatusFilter defines how to filter tasks by status.
type StatusFilter string

const (
	FilterActive  StatusFilter = "active"  // open + blocked (not done)
	FilterOpen    StatusFilter = "open"    // only open
	FilterBlocked StatusFilter = "blocked" // only blocked
	FilterDone    StatusFilter = "done"    // only done
	FilterAll     StatusFilter = "all"     // everything
)

// FilterByStatus returns tasks matching the given status filter.
func FilterByStatus(tasks []Task, filter StatusFilter) []Task {
	if filter == FilterAll {
		return tasks
	}

	var result []Task

	for _, t := range tasks {
		switch filter {
		case FilterActive:
			if t.Status != StatusDone {
				result = append(result, t)
			}
		case FilterOpen:
			if t.Status == StatusOpen {
				result = append(result, t)
			}
		case FilterBlocked:
			if t.Status == StatusBlocked {
				result = append(result, t)
			}
		case FilterDone:
			if t.Status == StatusDone {
				result = append(result, t)
			}
		default:
			// FilterAll is handled by early return above
		}
	}

	return result
}

// FilterByType returns tasks of the given type.
func FilterByType(tasks []Task, taskType TaskType) []Task {
	var result []Task

	for _, t := range tasks {
		if t.Type == taskType {
			result = append(result, t)
		}
	}

	return result
}

// FilterByTitle returns tasks whose title contains the query (case-insensitive).
func FilterByTitle(tasks []Task, query string) []Task {
	if query == "" {
		return tasks
	}

	lower := strings.ToLower(query)

	var result []Task

	for _, t := range tasks {
		if strings.Contains(strings.ToLower(t.Title), lower) {
			result = append(result, t)
		}
	}

	return result
}

// Counters holds summary counts for a set of tasks.
type Counters struct {
	Overdue int
	DueSoon int
	Open    int
	Blocked int
	Done    int
}

// GetCounters computes summary counts from a task slice.
func GetCounters(tasks []Task) Counters {
	var c Counters

	for _, t := range tasks {
		switch GetUrgency(t) {
		case UrgencyOverdue:
			c.Overdue++
		case UrgencyDueSoon:
			c.DueSoon++
		case UrgencyBlocked:
			c.Blocked++
		case UrgencyDone:
			c.Done++
		case UrgencyNormal:
			c.Open++
		}
	}

	return c
}

// GetTasksForRepo returns tasks for a specific repo plus global and review tasks.
// When querying global or reviews directly, returns only those.
func GetTasksForRepo(tasks []Task, repoID string) []Task {
	var result []Task

	for _, t := range tasks {
		if t.RepoID == repoID {
			result = append(result, t)
		}
	}

	return result
}
