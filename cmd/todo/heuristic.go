package main

import (
	"regexp"
	"strings"
)

// ParsedTask holds the result of parsing natural language into task fields.
type ParsedTask struct {
	Title       string
	Type        TaskType
	Priority    TaskPriority
	DueDate     string
	Description string
}

var dateRegex = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// HeuristicParse extracts task fields from natural language using keyword matching.
// This is the offline fallback when AI parsing via the q daemon is unavailable.
func HeuristicParse(text string) ParsedTask {
	lower := strings.ToLower(text)
	result := ParsedTask{
		Title:    text,
		Type:     TypeChore,
		Priority: PriorityMedium,
	}

	// Type detection
	switch {
	case containsAny(lower, "bug", "fix", "broken", "error", "crash", "failing"):
		result.Type = TypeBug
	case containsAny(lower, "add", "implement", "feature", "new", "create", "build"):
		result.Type = TypeFeature
	case containsAny(lower, "research", "investigate", "look into", "explore", "study", "learn"):
		result.Type = TypeResearch
	case containsAny(lower, "personal", "home", "errand", "buy", "groceries"):
		result.Type = TypePersonal
	case containsAny(lower, "review", "pr ", "pull request"):
		result.Type = TypeReview
	case containsAny(lower, "idea", "consider", "maybe", "think about"):
		result.Type = TypeResearch
	}

	// Priority detection
	switch {
	case containsAny(lower, "urgent", "critical", "asap", "blocking", "immediately"):
		result.Priority = PriorityHigh
	case containsAny(lower, "low priority", "nice to have", "eventually", "someday", "when free"):
		result.Priority = PriorityLow
	}

	// Date extraction
	if match := dateRegex.FindString(text); match != "" {
		result.DueDate = match
	}

	return result
}

func containsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}

	return false
}
