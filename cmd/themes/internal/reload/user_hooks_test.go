package reload

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestRunUserHooksInFilenameOrder verifies b7 invariants:
//   - .sh files under user .hooks/ are discovered
//   - they run in stable filename order
//   - non-executable .sh files are surfaced as warnings and skipped
//   - missing .hooks/ dir is a silent no-op
//   - README.md is not treated as a hook
func TestRunUserHooksInFilenameOrder(t *testing.T) {
	tmp := t.TempDir()

	// Point hooksDir() at our fixture.
	os.Setenv("XDG_CONFIG_HOME", tmp)
	t.Cleanup(func() { os.Unsetenv("XDG_CONFIG_HOME") })
	hooks := filepath.Join(tmp, "themes", ".hooks")
	os.MkdirAll(hooks, 0o755)

	// Marker file to prove hook fired.
	marker := filepath.Join(tmp, "marker")

	// Three executable hooks in reverse alphabetical order to prove sort.
	os.WriteFile(filepath.Join(hooks, "c-third.sh"),
		[]byte("#!/bin/sh\necho c >> "+marker+"\n"), 0o755)
	os.WriteFile(filepath.Join(hooks, "a-first.sh"),
		[]byte("#!/bin/sh\necho a >> "+marker+"\n"), 0o755)
	os.WriteFile(filepath.Join(hooks, "b-second.sh"),
		[]byte("#!/bin/sh\necho b >> "+marker+"\n"), 0o755)

	// Non-executable hook — must be surfaced as warning and skipped.
	os.WriteFile(filepath.Join(hooks, "d-inert.sh"),
		[]byte("#!/bin/sh\necho d >> "+marker+"\n"), 0o644)

	// README.md — must NOT be treated as a hook.
	os.WriteFile(filepath.Join(hooks, "README.md"), []byte("# docs\n"), 0o644)

	var stderr bytes.Buffer
	results := RunUserHooks(context.Background(), tmp, &stderr)

	// Should have 3 executed hooks (not 4, not 5).
	if len(results) != 3 {
		t.Fatalf("got %d user-hook results, want 3: %+v", len(results), results)
	}
	for i, expect := range []string{"user:a-first.sh", "user:b-second.sh", "user:c-third.sh"} {
		if results[i].Name != expect {
			t.Errorf("results[%d].Name = %q, want %q", i, results[i].Name, expect)
		}
	}

	// Non-executable warning must be in stderr.
	if !bytes.Contains(stderr.Bytes(), []byte("d-inert.sh is not executable")) {
		t.Errorf("no not-executable warning: %s", stderr.String())
	}

	// Marker file should have a\nb\nc\n exactly.
	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker missing: %v", err)
	}
	if string(got) != "a\nb\nc\n" {
		t.Errorf("marker = %q, want %q (proves order + skip)", got, "a\nb\nc\n")
	}
}

// TestRunUserHooksNoDirIsNoOp covers the "user has no .hooks/" case:
// silent nil return, no error.
func TestRunUserHooksNoDirIsNoOp(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	t.Cleanup(func() { os.Unsetenv("XDG_CONFIG_HOME") })

	var stderr bytes.Buffer
	results := RunUserHooks(context.Background(), tmp, &stderr)
	if len(results) != 0 {
		t.Errorf("no .hooks/ dir should yield 0 results, got %d", len(results))
	}
	if stderr.Len() != 0 {
		t.Errorf("no .hooks/ dir should be silent, got: %s", stderr.String())
	}
}
