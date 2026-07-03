package main

import (
	"testing"
)

func TestBuildPromptWithContext_NoContext(t *testing.T) {
	result := BuildPromptWithContext("", "hello world")
	if result != "hello world" {
		t.Errorf("expected plain prompt, got %q", result)
	}
}

func TestBuildPromptWithContext_WithContext(t *testing.T) {
	result := BuildPromptWithContext("error log here", "explain this")

	expected := "<context>\nerror log here\n</context>\n\nexplain this"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c.socketPath == "" {
		t.Error("expected non-empty socket path")
	}
}
