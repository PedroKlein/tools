package state

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSwapCurrentSymlinkDoesNotWriteState is the a4-live invariant:
// preview tier swaps the visible symlink but never mutates state.json.
// If this fails, the picker's ESC-revert semantics become subtly wrong
// because activeState().Theme starts tracking the previewed theme
// instead of the actually-committed one.
func TestSwapCurrentSymlinkDoesNotWriteState(t *testing.T) {
	orig := os.Getenv("XDG_STATE_HOME")
	tmp := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmp)
	t.Cleanup(func() { os.Setenv("XDG_STATE_HOME", orig) })

	a := filepath.Join(tmp, "themes", "A")
	b := filepath.Join(tmp, "themes", "B")
	if err := os.MkdirAll(filepath.Join(a, "derived"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(b, "derived"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Prime state to A via SetCurrent (writes state.json).
	if err := SetCurrent("A", a); err != nil {
		t.Fatal(err)
	}
	s, _ := Load()
	if s.Theme != "A" {
		t.Fatalf("prime state.Theme = %q, want A", s.Theme)
	}

	// Preview swap to B — state.json MUST NOT change.
	if err := SwapCurrentSymlink(b); err != nil {
		t.Fatal(err)
	}
	s, _ = Load()
	if s.Theme != "A" {
		t.Errorf("state.json mutated during preview: state.Theme = %q, want A", s.Theme)
	}
	if got := CurrentTarget(); got != b {
		t.Errorf("symlink did not swap: got %q want %q", got, b)
	}
}

// TestSwapCurrentSymlinkRejectsMissingDerived documents that preview
// mode still refuses to point at a theme with no derived/, so preview
// hooks never fire against an empty tree.
func TestSwapCurrentSymlinkRejectsMissingDerived(t *testing.T) {
	orig := os.Getenv("XDG_STATE_HOME")
	tmp := t.TempDir()
	os.Setenv("XDG_STATE_HOME", tmp)
	t.Cleanup(func() { os.Setenv("XDG_STATE_HOME", orig) })

	dir := filepath.Join(tmp, "themes", "no-derived")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := SwapCurrentSymlink(dir); err == nil {
		t.Fatal("expected error for theme with no derived/")
	}
}
