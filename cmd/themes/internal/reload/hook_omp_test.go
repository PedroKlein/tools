package reload

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHookOMPRewritesCurrentTheme(t *testing.T) {
	tmp := t.TempDir()
	themeDir := filepath.Join(tmp, "theme")
	if err := os.MkdirAll(filepath.Join(themeDir, "derived"), 0o750); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"name":"osaka-jade","colors":{"accent":"accent"},"vars":{"accent":"#549E6A"}}`)
	if err := os.WriteFile(filepath.Join(themeDir, "derived", "omp.json"), original, 0o600); err != nil {
		t.Fatal(err)
	}

	agentDir := filepath.Join(tmp, "omp", "agent")
	if err := os.MkdirAll(agentDir, 0o750); err != nil {
		t.Fatal(err)
	}
	origDir := ompAgentDir
	ompAgentDir = func() string { return agentDir }
	t.Cleanup(func() { ompAgentDir = origDir })

	if err := hookOMP(context.Background(), themeDir); err != nil {
		t.Fatalf("hookOMP: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(agentDir, "themes", "current.json"))
	if err != nil {
		t.Fatalf("missing current.json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if doc["name"] != "current" {
		t.Fatalf("name = %v, want current", doc["name"])
	}
	if doc["colors"] == nil || doc["vars"] == nil {
		t.Fatalf("theme payload dropped fields: %#v", doc)
	}
}

func TestHookOMPNoOpWithoutPayload(t *testing.T) {
	tmp := t.TempDir()
	themeDir := filepath.Join(tmp, "theme")
	if err := os.MkdirAll(filepath.Join(themeDir, "derived"), 0o750); err != nil {
		t.Fatal(err)
	}

	agentDir := filepath.Join(tmp, "omp", "agent")
	origDir := ompAgentDir
	ompAgentDir = func() string { return agentDir }
	t.Cleanup(func() { ompAgentDir = origDir })

	if err := hookOMP(context.Background(), themeDir); err != nil {
		t.Fatalf("hookOMP without payload: %v", err)
	}
	if _, err := os.Stat(filepath.Join(agentDir, "themes", "current.json")); err == nil {
		t.Fatal("wrote current.json despite missing payload")
	}
}
