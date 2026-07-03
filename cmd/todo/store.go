package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store manages task persistence in JSON files.
// Each file corresponds to a scope (repoId) and contains a JSON array of tasks.
type Store struct {
	root string // directory containing JSON files
}

// NewStore creates a store rooted at the given directory.
func NewStore(root string) *Store {
	return &Store{root: root}
}

// DefaultRoot returns the default store path (~/.config/todo/).
func DefaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "todo")
	}

	return filepath.Join(home, ".config", "todo")
}

// Root returns the store's root directory.
func (s *Store) Root() string {
	return s.root
}

// LoadScope reads all tasks for a given scope.
func (s *Store) LoadScope(scope string) ([]Task, error) {
	path := s.scopePath(scope)

	data, err := os.ReadFile(path) //nolint:gosec // path is constructed from trusted root and validated scope name
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("reading %s: %w", scope, err)
	}

	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", scope, err)
	}

	return tasks, nil
}

// LoadAll reads tasks from all scope files in the store directory.
func (s *Store) LoadAll() ([]Task, error) {
	scopes, err := s.ListScopes()
	if err != nil {
		return nil, err
	}

	var all []Task

	for _, scope := range scopes {
		tasks, err := s.LoadScope(scope)
		if err != nil {
			return nil, err
		}

		all = append(all, tasks...)
	}

	return all, nil
}

// Save persists tasks by splitting them into scope files by RepoID.
func (s *Store) Save(tasks []Task) error {
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return fmt.Errorf("creating store directory: %w", err)
	}

	// Group tasks by repoId
	groups := make(map[string][]Task)

	// Initialize with existing scopes so we clear empty ones
	scopes, _ := s.ListScopes()
	for _, scope := range scopes {
		groups[scope] = nil
	}

	for i := range tasks {
		scope := tasks[i].RepoID
		groups[scope] = append(groups[scope], tasks[i])
	}

	// Write each scope
	for scope, scopeTasks := range groups {
		if scopeTasks == nil {
			scopeTasks = []Task{}
		}

		if err := s.saveScope(scope, scopeTasks); err != nil {
			return err
		}
	}

	return nil
}

// SaveScope persists tasks for a single scope.
func (s *Store) SaveScope(scope string, tasks []Task) error {
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return fmt.Errorf("creating store directory: %w", err)
	}

	return s.saveScope(scope, tasks)
}

// ListScopes returns all scope names (derived from JSON filenames).
func (s *Store) ListScopes() ([]string, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("listing store: %w", err)
	}

	var scopes []string

	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".json") && !strings.HasPrefix(name, ".") {
			scopes = append(scopes, strings.TrimSuffix(name, ".json"))
		}
	}

	return scopes, nil
}

// NextID returns the next available ID for a given scope.
func (s *Store) NextID(scope string) (int, error) {
	tasks, err := s.LoadScope(scope)
	if err != nil {
		return 0, err
	}

	return nextIDFromTasks(tasks), nil
}

// nextIDFromTasks computes max(id) + 1 from a task slice.
func nextIDFromTasks(tasks []Task) int {
	if len(tasks) == 0 {
		return 1
	}

	maxID := 0
	for _, t := range tasks {
		if t.ID > maxID {
			maxID = t.ID
		}
	}

	return maxID + 1
}

func (s *Store) scopePath(scope string) string {
	return filepath.Join(s.root, scope+".json")
}

func (s *Store) saveScope(scope string, tasks []Task) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", scope, err)
	}

	path := s.scopePath(scope)
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", scope, err)
	}

	return nil
}
