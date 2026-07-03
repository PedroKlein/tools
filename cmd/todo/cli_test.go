package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func newTestCLI(t *testing.T) (*CLI, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	store := NewStore(dir)
	cli := NewCLI(store, "test-repo")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cli.SetOutput(stdout, stderr)

	return cli, stdout, stderr
}

func TestCLIAddQuick(t *testing.T) {
	cli, stdout, _ := newTestCLI(t)

	code := cli.Run("add", []string{"Fix the bug"}, false)
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "Task #1 created") {
		t.Errorf("output = %q, want task created message", output)
	}

	if !strings.Contains(output, "Fix the bug") {
		t.Errorf("output = %q, want title in output", output)
	}
}

func TestCLIAddWithFlags(t *testing.T) {
	cli, stdout, _ := newTestCLI(t)

	code := cli.Run("add", []string{"-t", "bug", "-p", "high", "-due", "2026-08-01", "Critical bug"}, true)
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var task Task
	if err := json.Unmarshal(stdout.Bytes(), &task); err != nil {
		t.Fatalf("JSON parse: %v\noutput: %s", err, stdout.String())
	}

	if task.Type != TypeBug {
		t.Errorf("Type = %q, want bug", task.Type)
	}

	if task.Priority != PriorityHigh {
		t.Errorf("Priority = %q, want high", task.Priority)
	}

	if task.DueDate != "2026-08-01" {
		t.Errorf("DueDate = %q, want 2026-08-01", task.DueDate)
	}
}

func TestCLIAddNoTitle(t *testing.T) {
	cli, _, _ := newTestCLI(t)

	code := cli.Run("add", []string{}, false)
	if code != ExitError {
		t.Errorf("exit code = %d, want 1 (error)", code)
	}
}

func TestCLIAddInvalidType(t *testing.T) {
	cli, _, _ := newTestCLI(t)

	code := cli.Run("add", []string{"-t", "invalid", "Test"}, false)
	if code != ExitError {
		t.Errorf("exit code = %d, want 1 (error)", code)
	}
}

func TestCLIListEmpty(t *testing.T) {
	cli, stdout, _ := newTestCLI(t)

	code := cli.Run("list", []string{}, false)
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if !strings.Contains(stdout.String(), "No tasks") {
		t.Errorf("output = %q, want 'No tasks' message", stdout.String())
	}
}

func TestCLIListJSON(t *testing.T) {
	cli, stdout, _ := newTestCLI(t)

	// Add a task first
	cli.Run("add", []string{"Test task"}, false)
	stdout.Reset()

	code := cli.Run("list", []string{"-s", "all"}, true)
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var tasks []Task
	if err := json.Unmarshal(stdout.Bytes(), &tasks); err != nil {
		t.Fatalf("JSON parse: %v\noutput: %s", err, stdout.String())
	}

	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

func TestCLIListFilters(t *testing.T) {
	cli, stdout, _ := newTestCLI(t)

	// Add tasks of different types
	cli.Run("add", []string{"-t", "bug", "Bug one"}, false)
	cli.Run("add", []string{"-t", "feature", "Feature one"}, false)
	stdout.Reset()

	// Filter by type
	code := cli.Run("list", []string{"-t", "bug", "-s", "all"}, true)
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var tasks []Task
	if err := json.Unmarshal(stdout.Bytes(), &tasks); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("expected 1 bug, got %d", len(tasks))
	}

	if tasks[0].Type != TypeBug {
		t.Errorf("Type = %q, want bug", tasks[0].Type)
	}
}

func TestCLIDone(t *testing.T) {
	cli, stdout, _ := newTestCLI(t)

	cli.Run("add", []string{"Task to complete"}, false)
	stdout.Reset()

	code := cli.Run("done", []string{"1"}, true)
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var task Task
	if err := json.Unmarshal(stdout.Bytes(), &task); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}

	if task.Status != StatusDone {
		t.Errorf("Status = %q, want done", task.Status)
	}
}

func TestCLIDoneNotFound(t *testing.T) {
	cli, _, _ := newTestCLI(t)

	code := cli.Run("done", []string{"999"}, false)
	if code != ExitNotFound {
		t.Errorf("exit code = %d, want 2 (not found)", code)
	}
}

func TestCLIUpdate(t *testing.T) {
	cli, stdout, _ := newTestCLI(t)

	cli.Run("add", []string{"Original title"}, false)
	stdout.Reset()

	code := cli.Run("update", []string{"-title", "Updated title", "-p", "high", "1"}, true)
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var task Task
	if err := json.Unmarshal(stdout.Bytes(), &task); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}

	if task.Title != "Updated title" {
		t.Errorf("Title = %q, want 'Updated title'", task.Title)
	}

	if task.Priority != PriorityHigh {
		t.Errorf("Priority = %q, want high", task.Priority)
	}
}

func TestCLIDelete(t *testing.T) {
	cli, stdout, _ := newTestCLI(t)

	cli.Run("add", []string{"To be deleted"}, false)
	stdout.Reset()

	code := cli.Run("delete", []string{"1"}, true)
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}

	// Verify it's gone
	stdout.Reset()
	cli.Run("list", []string{"-s", "all"}, true)

	var tasks []Task
	if err := json.Unmarshal(stdout.Bytes(), &tasks); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}

	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(tasks))
	}
}

func TestCLIDeleteNotFound(t *testing.T) {
	cli, _, _ := newTestCLI(t)

	code := cli.Run("delete", []string{"999"}, false)
	if code != ExitNotFound {
		t.Errorf("exit code = %d, want 2 (not found)", code)
	}
}

func TestCLIHelp(t *testing.T) {
	cli, stdout, _ := newTestCLI(t)

	code := cli.Run("help", nil, false)
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "todo") {
		t.Error("help output should contain 'todo'")
	}

	if !strings.Contains(output, "add") {
		t.Error("help output should mention 'add' command")
	}
}

func TestCLIUnknownCommand(t *testing.T) {
	cli, _, _ := newTestCLI(t)

	code := cli.Run("invalid", nil, false)
	if code != ExitError {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestParseRunArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCmd   string
		wantJSON  bool
		wantQuick bool
	}{
		{"empty", nil, "tui", false, false},
		{"add", []string{"add", "title"}, "add", false, false},
		{"list", []string{"list"}, "list", false, false},
		{"json flag", []string{"--json", "list"}, "list", true, false},
		{"quick flag", []string{"--quick", "add", "title"}, "add", false, true},
		{"help", []string{"help"}, "help", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRunArgs(tt.args)
			if got.Command != tt.wantCmd {
				t.Errorf("Command = %q, want %q", got.Command, tt.wantCmd)
			}

			if got.JSON != tt.wantJSON {
				t.Errorf("JSON = %v, want %v", got.JSON, tt.wantJSON)
			}

			if got.Quick != tt.wantQuick {
				t.Errorf("Quick = %v, want %v", got.Quick, tt.wantQuick)
			}
		})
	}
}

func TestCLIPersonalTaskRoutesToGlobal(t *testing.T) {
	cli, stdout, _ := newTestCLI(t)

	code := cli.Run("add", []string{"-t", "personal", "Buy milk"}, true)
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var task Task
	if err := json.Unmarshal(stdout.Bytes(), &task); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}

	if task.RepoID != GlobalRepoID {
		t.Errorf("RepoID = %q, want %q (personal tasks route to global)", task.RepoID, GlobalRepoID)
	}
}
