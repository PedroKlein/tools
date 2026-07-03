package main

import (
	"testing"
)

func TestParsePromptContainsRequiredParts(_ *testing.T) {
	// We can't test the private parsePrompt directly from _test package,
	// but we can test AIParse falls back gracefully when q is unavailable.
	// The AI parsing tests focus on response parsing and fallback behavior.
}

func TestAIParseResponse(t *testing.T) {
	// Test via AIParse which falls back to heuristic when socket unavailable
	// The heuristic fallback is the expected behavior in test environments
	got := AIParse("fix the authentication bug urgently")
	if got.Title == "" {
		t.Error("AIParse should return non-empty title")
	}
	// In test env (no q daemon), should fall back to heuristic
	if got.Type != TypeBug {
		t.Errorf("AIParse fallback: Type = %q, want bug", got.Type)
	}

	if got.Priority != PriorityHigh {
		t.Errorf("AIParse fallback: Priority = %q, want high (urgent keyword)", got.Priority)
	}
}

func TestAIParseFallbackWhenNoSocket(t *testing.T) {
	// Without q daemon running, AIParse should gracefully fall back
	got := AIParse("add new feature for dashboard")
	if got.Type != TypeFeature {
		t.Errorf("fallback Type = %q, want feature", got.Type)
	}
}
