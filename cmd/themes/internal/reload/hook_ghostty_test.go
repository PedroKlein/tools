package reload

import (
	"context"
	"testing"
)

// TestHookGhosttyIsNoOp verifies the b6 invariant: hookGhostty returns
// nil regardless of theme dir or context. This is intentional per the
// transparency investigation; see hook_ghostty.go for rationale.
func TestHookGhosttyIsNoOp(t *testing.T) {
	if err := hookGhostty(context.Background(), "/nonexistent"); err != nil {
		t.Errorf("hookGhostty should be no-op, got: %v", err)
	}
}
