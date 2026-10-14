package state_test

import (
	"testing"
	"time"

	"github.com/PedroKlein/tools/cmd/themes/internal/state"
)

func TestRapidCommitsHaveDistinctRevisions(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	if err := state.Save(state.State{Theme: "sample"}); err != nil {
		t.Fatal(err)
	}

	first, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}

	if saveErr := state.Save(state.State{Theme: "sample"}); saveErr != nil {
		t.Fatal(saveErr)
	}

	second, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}

	a, err := time.Parse(time.RFC3339Nano, first.ChangedAt)
	if err != nil {
		t.Fatal(err)
	}

	b, err := time.Parse(time.RFC3339Nano, second.ChangedAt)
	if err != nil {
		t.Fatal(err)
	}

	if !b.After(a) {
		t.Fatal("rapid commits share a revision and cannot reject stale login updates")
	}
}
