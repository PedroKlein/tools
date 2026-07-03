package main

import (
	"testing"
	"time"
)

func TestGetUrgencyDone(t *testing.T) {
	task := Task{Status: StatusDone}
	if got := GetUrgency(task); got != UrgencyDone {
		t.Errorf("done task: got %d, want UrgencyDone", got)
	}
}

func TestGetUrgencyBlocked(t *testing.T) {
	task := Task{Status: StatusBlocked}
	if got := GetUrgency(task); got != UrgencyBlocked {
		t.Errorf("blocked task: got %d, want UrgencyBlocked", got)
	}
}

func TestGetUrgencyOverdue(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	task := Task{Status: StatusOpen, DueDate: yesterday}
	if got := GetUrgency(task); got != UrgencyOverdue {
		t.Errorf("overdue task: got %d, want UrgencyOverdue", got)
	}
}

func TestGetUrgencyDueSoon(t *testing.T) {
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")

	task := Task{Status: StatusOpen, DueDate: tomorrow}
	if got := GetUrgency(task); got != UrgencyDueSoon {
		t.Errorf("due-soon task: got %d, want UrgencyDueSoon", got)
	}
}

func TestGetUrgencyNormal(t *testing.T) {
	nextWeek := time.Now().AddDate(0, 0, 7).Format("2006-01-02")

	task := Task{Status: StatusOpen, DueDate: nextWeek}
	if got := GetUrgency(task); got != UrgencyNormal {
		t.Errorf("normal task: got %d, want UrgencyNormal", got)
	}
}

func TestGetUrgencyNoDueDate(t *testing.T) {
	task := Task{Status: StatusOpen}
	if got := GetUrgency(task); got != UrgencyNormal {
		t.Errorf("no due date: got %d, want UrgencyNormal", got)
	}
}

func TestSortByUrgency(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")

	tasks := []Task{
		{ID: 1, Title: "Done", Status: StatusDone, Priority: PriorityHigh, CreatedAt: 100},
		{ID: 2, Title: "Normal", Status: StatusOpen, Priority: PriorityMedium, CreatedAt: 200},
		{ID: 3, Title: "Overdue", Status: StatusOpen, DueDate: yesterday, Priority: PriorityLow, CreatedAt: 300},
		{ID: 4, Title: "DueSoon", Status: StatusOpen, DueDate: tomorrow, Priority: PriorityHigh, CreatedAt: 400},
		{ID: 5, Title: "Blocked", Status: StatusBlocked, Priority: PriorityMedium, CreatedAt: 500},
	}

	SortByUrgency(tasks)

	expected := []string{"Overdue", "DueSoon", "Blocked", "Normal", "Done"}
	for i, title := range expected {
		if tasks[i].Title != title {
			t.Errorf("position %d: got %q, want %q", i, tasks[i].Title, title)
		}
	}
}

func TestSortByUrgencyPriorityTiebreak(t *testing.T) {
	tasks := []Task{
		{ID: 1, Title: "Low", Status: StatusOpen, Priority: PriorityLow, CreatedAt: 100},
		{ID: 2, Title: "High", Status: StatusOpen, Priority: PriorityHigh, CreatedAt: 200},
		{ID: 3, Title: "Medium", Status: StatusOpen, Priority: PriorityMedium, CreatedAt: 300},
	}

	SortByUrgency(tasks)

	expected := []string{"High", "Medium", "Low"}
	for i, title := range expected {
		if tasks[i].Title != title {
			t.Errorf("position %d: got %q, want %q", i, tasks[i].Title, title)
		}
	}
}

func TestFilterByStatus(t *testing.T) {
	tasks := []Task{
		{ID: 1, Status: StatusOpen},
		{ID: 2, Status: StatusBlocked},
		{ID: 3, Status: StatusDone},
		{ID: 4, Status: StatusOpen},
	}

	tests := []struct {
		filter StatusFilter
		want   int
	}{
		{FilterAll, 4},
		{FilterActive, 3},
		{FilterOpen, 2},
		{FilterBlocked, 1},
		{FilterDone, 1},
	}

	for _, tt := range tests {
		t.Run(string(tt.filter), func(t *testing.T) {
			got := FilterByStatus(tasks, tt.filter)
			if len(got) != tt.want {
				t.Errorf("FilterByStatus(%s): got %d, want %d", tt.filter, len(got), tt.want)
			}
		})
	}
}

func TestFilterByType(t *testing.T) {
	tasks := []Task{
		{ID: 1, Type: TypeFeature},
		{ID: 2, Type: TypeBug},
		{ID: 3, Type: TypeFeature},
	}

	features := FilterByType(tasks, TypeFeature)
	if len(features) != 2 {
		t.Errorf("expected 2 features, got %d", len(features))
	}

	bugs := FilterByType(tasks, TypeBug)
	if len(bugs) != 1 {
		t.Errorf("expected 1 bug, got %d", len(bugs))
	}
}

func TestFilterByTitle(t *testing.T) {
	tasks := []Task{
		{ID: 1, Title: "Fix rendering bug"},
		{ID: 2, Title: "Add dark mode"},
		{ID: 3, Title: "Fix authentication"},
	}

	got := FilterByTitle(tasks, "fix")
	if len(got) != 2 {
		t.Errorf("expected 2 matches for 'fix', got %d", len(got))
	}

	got = FilterByTitle(tasks, "")
	if len(got) != 3 {
		t.Errorf("empty query should return all, got %d", len(got))
	}
}

func TestGetCounters(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")

	tasks := []Task{
		{Status: StatusOpen, DueDate: yesterday}, // overdue
		{Status: StatusOpen, DueDate: tomorrow},  // due-soon
		{Status: StatusOpen},                     // open
		{Status: StatusBlocked},                  // blocked
		{Status: StatusDone},                     // done
	}

	c := GetCounters(tasks)
	if c.Overdue != 1 {
		t.Errorf("Overdue = %d, want 1", c.Overdue)
	}

	if c.DueSoon != 1 {
		t.Errorf("DueSoon = %d, want 1", c.DueSoon)
	}

	if c.Open != 1 {
		t.Errorf("Open = %d, want 1", c.Open)
	}

	if c.Blocked != 1 {
		t.Errorf("Blocked = %d, want 1", c.Blocked)
	}

	if c.Done != 1 {
		t.Errorf("Done = %d, want 1", c.Done)
	}
}

func TestGetTasksForRepo(t *testing.T) {
	tasks := []Task{
		{ID: 1, RepoID: "my-repo", Title: "Feature"},
		{ID: 2, RepoID: "global", Title: "Personal"},
		{ID: 3, RepoID: "reviews", Title: "Review"},
		{ID: 4, RepoID: "other-repo", Title: "Other"},
	}

	// Each scope only returns its own tasks
	got := GetTasksForRepo(tasks, "my-repo")
	if len(got) != 1 {
		t.Errorf("my-repo: expected 1 task, got %d", len(got))
	}

	got = GetTasksForRepo(tasks, "global")
	if len(got) != 1 {
		t.Errorf("global: expected 1 task, got %d", len(got))
	}

	got = GetTasksForRepo(tasks, "reviews")
	if len(got) != 1 {
		t.Errorf("reviews: expected 1 task, got %d", len(got))
	}

	got = GetTasksForRepo(tasks, "other-repo")
	if len(got) != 1 {
		t.Errorf("other-repo: expected 1 task, got %d", len(got))
	}
}
