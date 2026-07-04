package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/PedroKlein/tools/pkg/pirpc"
)

// parsePrompt builds the system prompt for AI task parsing.
func parsePrompt(text string) string {
	now := time.Now()
	isoDate := now.Format("2006-01-02")
	dayName := now.Format("Monday")
	dateStr := now.Format("January 2, 2006")

	return fmt.Sprintf(`Parse this task description into structured fields. Return ONLY valid JSON, no markdown fences.

Current date: %s, %s (%s)

Task: "%s"

Available types: ["feature", "bug", "chore", "research", "review", "personal"]
Available priorities: ["low", "medium", "high"]

Return a JSON object with these fields (omit fields you can't confidently infer):
{
  "title": "concise task title",
  "type": "one of the available types",
  "priority": "one of the available priorities",
  "dueDate": "YYYY-MM-DD if a date is mentioned, otherwise omit",
  "description": "concise 2-3 sentence description"
}

Rules:
- title: Clean, concise version of the task. Don't just echo the input.
- type: "bug" for fixing issues, "feature" for new capabilities, "chore" for maintenance, "research" for investigation, "review" for code/PR reviews, "personal" for non-work.
- priority: "high" for urgent/blocking, "low" for nice-to-have, "medium" for normal.
- dueDate: Only include if a specific date is mentioned or strongly implied. Use YYYY-MM-DD format. Resolve relative dates using the current date above.
- description: A concise 2-3 sentence summary that captures key details and intent.`, dayName, dateStr, isoDate, text)
}

// AIParse attempts to parse a task description using the q daemon for AI inference.
// Falls back to heuristic parsing if the daemon is unavailable or returns invalid data.
func AIParse(text string) ParsedTask {
	parsed, _ := AIParseWithStatus(text)
	return parsed
}

// AIParseWithStatus is like AIParse but also reports whether AI was actually used.
func AIParseWithStatus(text string) (ParsedTask, bool) {
	if !pirpc.IsAvailable() {
		return HeuristicParse(text), false
	}

	prompt := parsePrompt(text)
	response, err := pirpc.PromptFresh(prompt)
	// Always reset session after parsing so q starts clean next time
	defer func() { _ = pirpc.ResetSession() }()

	if err != nil {
		return HeuristicParse(text), false
	}

	parsed, err := parseAIResponse(response)
	if err != nil {
		return HeuristicParse(text), false
	}

	return parsed, true
}

// parseAIResponse extracts a ParsedTask from the AI's JSON response.
func parseAIResponse(response string) (ParsedTask, error) {
	// Find JSON object in response (handles potential markdown fences)
	start := strings.Index(response, "{")

	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end <= start {
		return ParsedTask{}, errors.New("no JSON object found in response")
	}

	jsonStr := response[start : end+1]

	var raw struct {
		Title       string `json:"title"`
		Type        string `json:"type"`
		Priority    string `json:"priority"`
		DueDate     string `json:"dueDate"`
		Description string `json:"description"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return ParsedTask{}, fmt.Errorf("parsing JSON: %w", err)
	}

	result := ParsedTask{
		Title:       raw.Title,
		Type:        TypeChore,
		Priority:    PriorityMedium,
		Description: raw.Description,
	}

	if IsValidType(TaskType(raw.Type)) {
		result.Type = TaskType(raw.Type)
	}

	if IsValidPriority(TaskPriority(raw.Priority)) {
		result.Priority = TaskPriority(raw.Priority)
	}

	if dateRegex.MatchString(raw.DueDate) {
		result.DueDate = raw.DueDate
	}

	if result.Title == "" {
		return ParsedTask{}, errors.New("AI returned empty title")
	}

	return result, nil
}
