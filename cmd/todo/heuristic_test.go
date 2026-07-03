package main

import (
	"testing"
)

func TestHeuristicParseBug(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"fix rendering bug on dashboard"},
		{"broken authentication flow"},
		{"error in payment processing"},
		{"crash when clicking submit"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := HeuristicParse(tt.input)
			if got.Type != TypeBug {
				t.Errorf("HeuristicParse(%q).Type = %q, want bug", tt.input, got.Type)
			}
		})
	}
}

func TestHeuristicParseFeature(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"add dark mode support"},
		{"implement caching layer"},
		{"create new API endpoint"},
		{"build user dashboard"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := HeuristicParse(tt.input)
			if got.Type != TypeFeature {
				t.Errorf("HeuristicParse(%q).Type = %q, want feature", tt.input, got.Type)
			}
		})
	}
}

func TestHeuristicParseResearch(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"research caching strategies"},
		{"investigate memory leak"},
		{"look into alternative libraries"},
		{"explore GraphQL options"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := HeuristicParse(tt.input)
			if got.Type != TypeResearch {
				t.Errorf("HeuristicParse(%q).Type = %q, want research", tt.input, got.Type)
			}
		})
	}
}

func TestHeuristicParsePersonal(t *testing.T) {
	got := HeuristicParse("buy groceries for the week")
	if got.Type != TypePersonal {
		t.Errorf("Type = %q, want personal", got.Type)
	}
}

func TestHeuristicParsePriority(t *testing.T) {
	tests := []struct {
		input string
		want  TaskPriority
	}{
		{"urgent fix for production", PriorityHigh},
		{"critical bug in auth", PriorityHigh},
		{"blocking deployment", PriorityHigh},
		{"low priority cleanup", PriorityLow},
		{"nice to have feature", PriorityLow},
		{"eventually refactor this", PriorityLow},
		{"regular task", PriorityMedium},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := HeuristicParse(tt.input)
			if got.Priority != tt.want {
				t.Errorf("HeuristicParse(%q).Priority = %q, want %q", tt.input, got.Priority, tt.want)
			}
		})
	}
}

func TestHeuristicParseDateExtraction(t *testing.T) {
	got := HeuristicParse("submit report by 2026-07-15")
	if got.DueDate != "2026-07-15" {
		t.Errorf("DueDate = %q, want 2026-07-15", got.DueDate)
	}
}

func TestHeuristicParseNoDate(t *testing.T) {
	got := HeuristicParse("fix the bug")
	if got.DueDate != "" {
		t.Errorf("DueDate = %q, want empty", got.DueDate)
	}
}

func TestHeuristicParseTitle(t *testing.T) {
	got := HeuristicParse("fix the rendering bug")
	if got.Title != "fix the rendering bug" {
		t.Errorf("Title = %q, want original text", got.Title)
	}
}

func TestHeuristicParseDefaultChore(t *testing.T) {
	got := HeuristicParse("update dependencies")
	if got.Type != TypeChore {
		t.Errorf("Type = %q, want chore for ambiguous input", got.Type)
	}
}
