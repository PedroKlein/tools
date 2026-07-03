package main

import (
	"fmt"
	"strings"
)

// TabModel manages the repo tab bar.
type TabModel struct {
	scopes []string
	counts map[string]int
	active int
	width  int
	repoID string // user's current repo (shown first)
}

// NewTabModel creates a tab model discovering scopes from the store.
func NewTabModel(store *Store, repoID string) TabModel {
	scopes := discoverScopes(store, repoID)
	counts := computeCounts(store, scopes)

	return TabModel{
		scopes: scopes,
		counts: counts,
		active: 0,
		repoID: repoID,
	}
}

// ActiveScope returns the scope name for the currently active tab.
func (t *TabModel) ActiveScope() string {
	if t.active >= 0 && t.active < len(t.scopes) {
		return t.scopes[t.active]
	}

	return GlobalRepoID
}

// Next moves to the next tab.
func (t *TabModel) Next() {
	t.active = (t.active + 1) % len(t.scopes)
}

// Prev moves to the previous tab.
func (t *TabModel) Prev() {
	t.active = (t.active - 1 + len(t.scopes)) % len(t.scopes)
}

// UpdateCounts refreshes task counts per scope.
func (t *TabModel) UpdateCounts(store *Store) {
	t.counts = computeCounts(store, t.scopes)
}

// View renders the tab bar.
func (t TabModel) View() string {
	var tabs []string

	for i, scope := range t.scopes {
		label := shortScopeName(scope)
		count := t.counts[scope]
		text := fmt.Sprintf("%s (%d)", label, count)

		if i == t.active {
			tabs = append(tabs, StyleTabActive.Render("● "+text))
		} else {
			tabs = append(tabs, StyleTabInactive.Render("○ "+text))
		}
	}

	bar := strings.Join(tabs, " ")

	return StyleTabBar.Width(t.width).Render(bar)
}

func discoverScopes(store *Store, repoID string) []string {
	fileScopes, _ := store.ListScopes()

	// Build ordered list: current repo first, then others, with "all" at end
	seen := make(map[string]bool)

	var scopes []string

	// Current repo first
	scopes = append(scopes, repoID)
	seen[repoID] = true

	// Global and reviews
	for _, s := range []string{GlobalRepoID, ReviewRepoID} {
		if !seen[s] {
			scopes = append(scopes, s)
			seen[s] = true
		}
	}

	// Other discovered scopes
	for _, s := range fileScopes {
		if !seen[s] {
			scopes = append(scopes, s)
			seen[s] = true
		}
	}

	return scopes
}

func computeCounts(store *Store, scopes []string) map[string]int {
	counts := make(map[string]int)
	all, _ := store.LoadAll()

	for _, scope := range scopes {
		tasks := GetTasksForRepo(all, scope)
		active := FilterByStatus(tasks, FilterActive)
		counts[scope] = len(active)
	}

	return counts
}

func shortScopeName(scope string) string {
	switch scope {
	case GlobalRepoID:
		return "personal"
	case WorkRepoID:
		return "work"
	case ReviewRepoID:
		return "reviews"
	default:
		// Split on "__" separator: "pedroklein__dotfiles" → "dotfiles"
		if idx := strings.Index(scope, "__"); idx != -1 && idx+2 < len(scope) {
			return scope[idx+2:]
		}

		return scope
	}
}
