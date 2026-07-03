// Package todo provides the domain model and store for the todo CLI tool.
// It manages tasks persisted as JSON files in ~/.config/todo/.
package main

import (
	"slices"
	"time"
)

// Scope constants for routing tasks to the correct file.
const (
	GlobalRepoID = "global"
	WorkRepoID   = "work"
	ReviewRepoID = "reviews"
)

// TaskType categorizes a task.
type TaskType string

const (
	TypeFeature  TaskType = "feature"
	TypeBug      TaskType = "bug"
	TypeChore    TaskType = "chore"
	TypeResearch TaskType = "research"
	TypeReview   TaskType = "review"
	TypePersonal TaskType = "personal"
)

// ValidTypes lists all valid task types.
var ValidTypes = []TaskType{TypeFeature, TypeBug, TypeChore, TypeResearch, TypeReview, TypePersonal}

// IsValidType returns true if t is a recognized task type.
func IsValidType(t TaskType) bool {
	return slices.Contains(ValidTypes, t)
}

// TaskPriority represents urgency.
type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
)

// ValidPriorities lists all valid priorities.
var ValidPriorities = []TaskPriority{PriorityLow, PriorityMedium, PriorityHigh}

// IsValidPriority returns true if p is a recognized priority.
func IsValidPriority(p TaskPriority) bool {
	return slices.Contains(ValidPriorities, p)
}

// TaskStatus represents the current state.
type TaskStatus string

const (
	StatusOpen    TaskStatus = "open"
	StatusBlocked TaskStatus = "blocked"
	StatusDone    TaskStatus = "done"
)

// ValidStatuses lists all valid statuses.
var ValidStatuses = []TaskStatus{StatusOpen, StatusBlocked, StatusDone}

// IsValidStatus returns true if s is a recognized status.
func IsValidStatus(s TaskStatus) bool {
	return slices.Contains(ValidStatuses, s)
}

// PrMeta holds GitHub PR metadata for review tasks.
type PrMeta struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	State  string `json:"state"`
	Branch string `json:"branch"`
	Host   string `json:"host"`
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
}

// Task is the core data model.
type Task struct {
	ID          int          `json:"id"`
	Title       string       `json:"title"`
	Status      TaskStatus   `json:"status"`
	Type        TaskType     `json:"type"`
	Priority    TaskPriority `json:"priority"`
	DueDate     string       `json:"dueDate,omitempty"`
	Description string       `json:"description,omitempty"`
	URL         string       `json:"url,omitempty"`
	PrMeta      *PrMeta      `json:"prMeta,omitempty"`
	Note        string       `json:"note,omitempty"`
	RepoID      string       `json:"repoId"`
	CreatedAt   int64        `json:"createdAt"`
	UpdatedAt   int64        `json:"updatedAt"`
}

// GetTaskRepoID returns the effective repoId for a task based on its type.
// Personal tasks → GlobalRepoID, Review tasks → ReviewRepoID, others → currentRepoID.
func GetTaskRepoID(taskType TaskType, currentRepoID string) string {
	switch taskType {
	case TypePersonal:
		return GlobalRepoID
	case TypeReview:
		return ReviewRepoID
	default:
		return currentRepoID
	}
}

// IsGlobalType returns true if the type always routes to a global scope.
func IsGlobalType(t TaskType) bool {
	return t == TypePersonal || t == TypeReview
}

// NewTask creates a task with sensible defaults.
func NewTask(id int, title, repoID string, taskType TaskType) Task {
	now := time.Now().UnixMilli()
	effectiveRepo := GetTaskRepoID(taskType, repoID)

	return Task{
		ID:        id,
		Title:     title,
		Status:    StatusOpen,
		Type:      taskType,
		Priority:  PriorityMedium,
		RepoID:    effectiveRepo,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
