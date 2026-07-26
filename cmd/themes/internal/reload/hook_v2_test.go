package reload

import (
	"context"
	"errors"
	"testing"
)

// TestFilterHooksHonorsNewShape covers the b0 invariant: a Hook that
// sets any of RunPreview/RunCommit/OS uses new-shape gating; LiveApply
// is ignored on that Hook. Legacy hooks still honor LiveApply.
func TestFilterHooksHonorsNewShape(t *testing.T) {
	orig := registry
	t.Cleanup(func() { registry = orig })

	registry = []Hook{
		{Name: "legacy-preview", LiveApply: true, Kind: KindNoop},
		{Name: "legacy-commit", LiveApply: false, Kind: KindNoop},
		{Name: "new-both", RunPreview: true, RunCommit: true,
			Fn: func(ctx context.Context, dir string) error { return nil }},
		{Name: "new-preview-only", RunPreview: true, RunCommit: false,
			Fn: func(ctx context.Context, dir string) error { return nil }},
		{Name: "new-commit-only", RunPreview: false, RunCommit: true,
			Fn: func(ctx context.Context, dir string) error { return nil }},
	}

	// Preview mode: liveApply=true → new-preview-only, new-both, legacy-preview.
	preview := FilterHooks(nil, true)
	previewNames := names(preview)
	wantPreview := []string{"legacy-preview", "new-both", "new-preview-only"}
	if !equalUnordered(previewNames, wantPreview) {
		t.Errorf("preview filter got %v, want %v", previewNames, wantPreview)
	}

	// Commit mode: liveApply=false → new-both, new-commit-only, both legacy.
	commit := FilterHooks(nil, false)
	commitNames := names(commit)
	wantCommit := []string{"legacy-preview", "legacy-commit", "new-both", "new-commit-only"}
	if !equalUnordered(commitNames, wantCommit) {
		t.Errorf("commit filter got %v, want %v", commitNames, wantCommit)
	}
}

// TestFilterHooksSkipList covers b0's contract that THEME_SKIP_HOOKS
// filters both new-shape and legacy entries by Name.
func TestFilterHooksSkipList(t *testing.T) {
	orig := registry
	t.Cleanup(func() { registry = orig })

	registry = []Hook{
		{Name: "keep-legacy", LiveApply: true, Kind: KindNoop},
		{Name: "skip-legacy", LiveApply: true, Kind: KindNoop},
		{Name: "keep-new", RunPreview: true, RunCommit: true,
			Fn: func(ctx context.Context, dir string) error { return nil }},
		{Name: "skip-new", RunPreview: true, RunCommit: true,
			Fn: func(ctx context.Context, dir string) error { return nil }},
	}
	got := names(FilterHooks(map[string]bool{"skip-legacy": true, "skip-new": true}, true))
	want := []string{"keep-legacy", "keep-new"}
	if !equalUnordered(got, want) {
		t.Errorf("skip list filter got %v, want %v", got, want)
	}
}

// TestOSMatchesNewShapePreferred covers b0: Hook.OS ("darwin"/"linux")
// takes precedence over the legacy darwinOnly name map.
func TestOSMatchesNewShapePreferred(t *testing.T) {
	origGOOS := runtimeGOOS
	t.Cleanup(func() { runtimeGOOS = origGOOS })

	// Force runtime to look like darwin for these assertions.
	runtimeGOOS = func() string { return "darwin" }

	// New-shape darwin hook: passes on darwin.
	if !osMatches(Hook{Name: "any-name", OS: "darwin"}) {
		t.Error("new-shape darwin hook rejected on darwin")
	}
	// New-shape linux hook: rejected on darwin.
	if osMatches(Hook{Name: "any-name", OS: "linux"}) {
		t.Error("new-shape linux hook accepted on darwin")
	}
	// New-shape no-OS hook: any.
	if !osMatches(Hook{Name: "any-name"}) {
		t.Error("new-shape no-OS hook rejected on darwin")
	}
	// Legacy darwin-only via name.
	if !osMatches(Hook{Name: "sketchybar"}) {
		t.Error("legacy sketchybar rejected on darwin")
	}

	runtimeGOOS = func() string { return "linux" }
	if osMatches(Hook{Name: "sketchybar"}) {
		t.Error("legacy sketchybar accepted on linux")
	}
	if osMatches(Hook{OS: "darwin"}) {
		t.Error("new-shape darwin accepted on linux")
	}
}

// TestRegistryIsMutableVar verifies the b0 constraint: registry is a
// var (not a func) so tests can override it via package-level
// substitution. Compilation of this test file is the assertion.
func TestRegistryIsMutableVar(t *testing.T) {
	orig := registry
	t.Cleanup(func() { registry = orig })
	registry = []Hook{{Name: "test-only", Fn: dummyFn}}
	if len(FilterHooks(nil, false)) != 1 {
		t.Error("substituted registry not honored")
	}
}

// dummyFn is a valid Fn for tests; a hasNewShape-only entry needs at
// least one new-shape field set. RunCommit=true triggers hasNewShape.
func dummyFn(ctx context.Context, dir string) error { return nil }

// Sanity: hasNewShape returns false for a bare hook with just Name.
func TestHasNewShapeDefaultFalse(t *testing.T) {
	if (Hook{Name: "bare"}).hasNewShape() {
		t.Error("bare Hook flagged as new shape")
	}
	if !(Hook{Name: "with-preview", RunPreview: true}).hasNewShape() {
		t.Error("RunPreview should flip shape")
	}
	if !(Hook{Name: "with-commit", RunCommit: true}).hasNewShape() {
		t.Error("RunCommit should flip shape")
	}
	if !(Hook{Name: "with-os", OS: "darwin"}).hasNewShape() {
		t.Error("OS should flip shape")
	}
	// Fn alone must NOT flip because legacy KindInline also has Fn.
	if (Hook{Name: "legacy-inline", Fn: func(context.Context, string) error { return errors.New("x") }}).hasNewShape() {
		t.Error("Fn alone should NOT flip shape (legacy KindInline collision)")
	}
}

// helpers -----------------------------------------------------------

func names(hooks []Hook) []string {
	out := make([]string, len(hooks))
	for i, h := range hooks {
		out[i] = h.Name
	}
	return out
}

func equalUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, v := range seen {
		if v != 0 {
			return false
		}
	}
	return true
}
