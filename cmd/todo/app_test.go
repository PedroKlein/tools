package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestApp(t *testing.T) App {
	t.Helper()
	dir := t.TempDir()
	store := NewStore(dir)

	// Seed some tasks
	tasks := []Task{
		NewTask(1, "Feature task", "test-repo", TypeFeature),
		NewTask(2, "Bug task", "test-repo", TypeBug),
		NewTask(3, "Done task", "test-repo", TypeChore),
	}
	tasks[2].Status = StatusDone

	if err := store.Save(tasks); err != nil {
		t.Fatal(err)
	}

	app := NewApp(store, "test-repo")
	// Simulate window size
	updated, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	a, ok := updated.(App)
	if !ok {
		t.Fatal("expected App model")
	}

	return a
}

func TestAppQuit(t *testing.T) {
	app := newTestApp(t)

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestAppCtrlCQuit(t *testing.T) {
	app := newTestApp(t)

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command from ctrl+c")
	}
}

func TestAppResize(t *testing.T) {
	app := newTestApp(t)

	updated, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	a, ok := updated.(App)
	if !ok {
		t.Fatal("expected App model")
	}
	// Should not panic
	_ = a.View()
}

func TestAppViewNotEmpty(t *testing.T) {
	app := newTestApp(t)

	view := app.View()
	if len(view) == 0 {
		t.Error("View() should not be empty")
	}
}
