package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreLoadEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	tasks, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestStoreLoadScopeNotExist(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	tasks, err := store.LoadScope("nonexistent")
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}

	if tasks != nil {
		t.Errorf("expected nil, got %v", tasks)
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	tasks := []Task{
		NewTask(1, "Task one", "my-repo", TypeFeature),
		NewTask(2, "Task two", "my-repo", TypeBug),
	}

	if err := store.Save(tasks); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.LoadScope("my-repo")
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(loaded))
	}

	if loaded[0].Title != "Task one" {
		t.Errorf("task 0 title = %q, want %q", loaded[0].Title, "Task one")
	}

	if loaded[1].Title != "Task two" {
		t.Errorf("task 1 title = %q, want %q", loaded[1].Title, "Task two")
	}
}

func TestStoreMultiScope(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	tasks := []Task{
		NewTask(1, "Feature", "repo-a", TypeFeature),
		NewTask(1, "Personal", "global", TypePersonal),
		NewTask(1, "Review PR", "reviews", TypeReview),
	}

	if err := store.Save(tasks); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Should create three separate files
	scopes, err := store.ListScopes()
	if err != nil {
		t.Fatalf("ListScopes: %v", err)
	}

	if len(scopes) != 3 {
		t.Errorf("expected 3 scopes, got %d: %v", len(scopes), scopes)
	}

	// Verify each scope has exactly one task
	for _, scope := range []string{"repo-a", "global", "reviews"} {
		loaded, err := store.LoadScope(scope)
		if err != nil {
			t.Fatalf("LoadScope(%s): %v", scope, err)
		}

		if len(loaded) != 1 {
			t.Errorf("scope %s: expected 1 task, got %d", scope, len(loaded))
		}
	}
}

func TestStoreNextID(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Empty scope → ID 1
	id, err := store.NextID("empty-repo")
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}

	if id != 1 {
		t.Errorf("empty scope: expected ID 1, got %d", id)
	}

	// Save some tasks
	tasks := []Task{
		{ID: 3, Title: "A", RepoID: "my-repo", Status: StatusOpen, Type: TypeChore, Priority: PriorityMedium, CreatedAt: 1, UpdatedAt: 1},
		{ID: 7, Title: "B", RepoID: "my-repo", Status: StatusOpen, Type: TypeChore, Priority: PriorityMedium, CreatedAt: 1, UpdatedAt: 1},
	}

	err = store.Save(tasks)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// NextID should be max+1 = 8
	id, err = store.NextID("my-repo")
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}

	if id != 8 {
		t.Errorf("expected ID 8, got %d", id)
	}
}

func TestStoreLoadAll(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	tasks := []Task{
		NewTask(1, "A", "repo-a", TypeFeature),
		NewTask(2, "B", "repo-a", TypeBug),
		NewTask(1, "C", "repo-b", TypeChore),
	}

	if err := store.Save(tasks); err != nil {
		t.Fatalf("Save: %v", err)
	}

	all, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(all))
	}
}

func TestStoreCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "path")
	store := NewStore(dir)

	task := NewTask(1, "Test", "repo", TypeChore)
	if err := store.Save([]Task{task}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("store directory should be created")
	}
}

func TestStoreListScopes(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Write two scope files
	tasks := []Task{
		NewTask(1, "A", "alpha", TypeFeature),
		NewTask(1, "B", "beta", TypeBug),
	}
	if err := store.Save(tasks); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Add a hidden file (should be ignored)
	if err := os.WriteFile(filepath.Join(dir, ".migrated"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	scopes, err := store.ListScopes()
	if err != nil {
		t.Fatalf("ListScopes: %v", err)
	}

	if len(scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d: %v", len(scopes), scopes)
	}
}
