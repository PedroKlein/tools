package main

import (
	"encoding/json"
	"testing"
)

func TestGetTaskRepoID(t *testing.T) {
	tests := []struct {
		name      string
		taskType  TaskType
		currentID string
		want      string
	}{
		{"personal routes to global", TypePersonal, "pedroklein-dotfiles", GlobalRepoID},
		{"review routes to reviews", TypeReview, "pedroklein-dotfiles", ReviewRepoID},
		{"feature uses current", TypeFeature, "pedroklein-dotfiles", "pedroklein-dotfiles"},
		{"bug uses current", TypeBug, "start-key-manager", "start-key-manager"},
		{"chore uses current", TypeChore, "my-repo", "my-repo"},
		{"research uses current", TypeResearch, "my-repo", "my-repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetTaskRepoID(tt.taskType, tt.currentID)
			if got != tt.want {
				t.Errorf("GetTaskRepoID(%q, %q) = %q, want %q", tt.taskType, tt.currentID, got, tt.want)
			}
		})
	}
}

func TestIsGlobalType(t *testing.T) {
	tests := []struct {
		taskType TaskType
		want     bool
	}{
		{TypePersonal, true},
		{TypeReview, true},
		{TypeFeature, false},
		{TypeBug, false},
		{TypeChore, false},
		{TypeResearch, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.taskType), func(t *testing.T) {
			if got := IsGlobalType(tt.taskType); got != tt.want {
				t.Errorf("IsGlobalType(%q) = %v, want %v", tt.taskType, got, tt.want)
			}
		})
	}
}

func TestNewTask(t *testing.T) {
	task := NewTask(1, "Fix bug", "pedroklein-dotfiles", TypeBug)

	if task.ID != 1 {
		t.Errorf("ID = %d, want 1", task.ID)
	}

	if task.Title != "Fix bug" {
		t.Errorf("Title = %q, want %q", task.Title, "Fix bug")
	}

	if task.Status != StatusOpen {
		t.Errorf("Status = %q, want %q", task.Status, StatusOpen)
	}

	if task.Type != TypeBug {
		t.Errorf("Type = %q, want %q", task.Type, TypeBug)
	}

	if task.Priority != PriorityMedium {
		t.Errorf("Priority = %q, want %q", task.Priority, PriorityMedium)
	}

	if task.RepoID != "pedroklein-dotfiles" {
		t.Errorf("RepoID = %q, want pedroklein-dotfiles", task.RepoID)
	}

	if task.CreatedAt == 0 {
		t.Error("CreatedAt should be set")
	}
}

func TestNewTaskPersonalRoutesToGlobal(t *testing.T) {
	task := NewTask(1, "Groceries", "some-repo", TypePersonal)
	if task.RepoID != GlobalRepoID {
		t.Errorf("RepoID = %q, want %q", task.RepoID, GlobalRepoID)
	}
}

func TestNewTaskReviewRoutesToReviews(t *testing.T) {
	task := NewTask(1, "Review PR", "some-repo", TypeReview)
	if task.RepoID != ReviewRepoID {
		t.Errorf("RepoID = %q, want %q", task.RepoID, ReviewRepoID)
	}
}

func TestTaskJSONRoundTrip(t *testing.T) {
	task := NewTask(5, "Test task", "my-repo", TypeFeature)
	task.Description = "A description"
	task.DueDate = "2026-07-15"
	task.PrMeta = &PrMeta{
		Title:  "Fix thing",
		Author: "user1",
		State:  "open",
		Host:   "github.com",
		Owner:  "org",
		Repo:   "repo",
		Number: 42,
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Task
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != task.ID {
		t.Errorf("ID: got %d, want %d", decoded.ID, task.ID)
	}

	if decoded.Title != task.Title {
		t.Errorf("Title: got %q, want %q", decoded.Title, task.Title)
	}

	if decoded.DueDate != task.DueDate {
		t.Errorf("DueDate: got %q, want %q", decoded.DueDate, task.DueDate)
	}

	if decoded.PrMeta == nil {
		t.Fatal("PrMeta should not be nil")
	}

	if decoded.PrMeta.Number != 42 {
		t.Errorf("PrMeta.Number: got %d, want 42", decoded.PrMeta.Number)
	}
}

func TestTaskJSONOmitsEmpty(t *testing.T) {
	task := NewTask(1, "Simple", "repo", TypeChore)

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	str := string(data)
	if contains(str, "dueDate") {
		t.Error("empty dueDate should be omitted from JSON")
	}

	if contains(str, "description") {
		t.Error("empty description should be omitted from JSON")
	}

	if contains(str, "prMeta") {
		t.Error("nil prMeta should be omitted from JSON")
	}
}

func TestIsValidType(t *testing.T) {
	if !IsValidType(TypeFeature) {
		t.Error("feature should be valid")
	}

	if IsValidType("invalid") {
		t.Error("invalid should not be valid")
	}
}

func TestIsValidPriority(t *testing.T) {
	if !IsValidPriority(PriorityHigh) {
		t.Error("high should be valid")
	}

	if IsValidPriority("critical") {
		t.Error("critical should not be valid")
	}
}

func TestIsValidStatus(t *testing.T) {
	if !IsValidStatus(StatusOpen) {
		t.Error("open should be valid")
	}

	if IsValidStatus("archived") {
		t.Error("archived should not be valid")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}
