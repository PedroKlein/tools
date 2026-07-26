package reload

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestHookPiRewritesNameField covers the primary invariant: after
// hookPi runs, each installed profile's themes/current.json has
// .name == "current" and no other fields dropped.
func TestHookPiRewritesNameField(t *testing.T) {
	tmp := t.TempDir()

	// Fixture: a theme with derived/pi.json.
	themeDir := filepath.Join(tmp, "theme")
	os.MkdirAll(filepath.Join(themeDir, "derived"), 0o755)
	src := filepath.Join(themeDir, "derived", "pi.json")
	original := []byte(`{"name":"osaka-jade","palette":{"fg":"#ccc","bg":"#111"},"description":"jade"}`)
	if err := os.WriteFile(src, original, 0o644); err != nil {
		t.Fatal(err)
	}

	// Fixture: two profile dirs (simulate agent + agent-quick).
	profs := []string{filepath.Join(tmp, "pi", "agent"), filepath.Join(tmp, "pi", "agent-quick")}
	for _, p := range profs {
		os.MkdirAll(p, 0o755)
	}
	// Third profile dir does NOT exist — must be silently skipped.
	origDirs := piProfileDirs
	piProfileDirs = func() []string {
		return []string{profs[0], profs[1], filepath.Join(tmp, "pi", "not-installed")}
	}
	t.Cleanup(func() { piProfileDirs = origDirs })

	if err := hookPi(context.Background(), themeDir); err != nil {
		t.Fatalf("hookPi: %v", err)
	}

	for _, p := range profs {
		dst := filepath.Join(p, "themes", "current.json")
		b, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("profile %s missing current.json: %v", p, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatalf("profile %s: invalid JSON: %v", p, err)
		}
		if doc["name"] != "current" {
			t.Errorf("profile %s: name = %v, want 'current'", p, doc["name"])
		}
		if doc["description"] != "jade" {
			t.Errorf("profile %s: description dropped: %v", p, doc)
		}
	}

	// Non-installed profile: nothing written.
	notInstalled := filepath.Join(tmp, "pi", "not-installed", "themes", "current.json")
	if _, err := os.Stat(notInstalled); err == nil {
		t.Errorf("wrote into non-installed profile dir: %s", notInstalled)
	}
}

// TestHookPiAtomicOnMidWriteError verifies the AC: even if writing to
// one profile fails, other profiles still get updated and pi never sees
// a corrupt current.json. Simulates a mid-write crash by using a
// read-only target for the "corrupt" profile.
func TestHookPiAtomicOnMidWriteError(t *testing.T) {
	tmp := t.TempDir()

	themeDir := filepath.Join(tmp, "theme")
	os.MkdirAll(filepath.Join(themeDir, "derived"), 0o755)
	os.WriteFile(filepath.Join(themeDir, "derived", "pi.json"),
		[]byte(`{"name":"seed"}`), 0o644)

	// good profile: normal dir.
	good := filepath.Join(tmp, "pi", "good")
	os.MkdirAll(good, 0o755)

	// bad profile: pre-seed a current.json, then make themes/ read-only
	// so temp+rename fails. current.json must remain untouched (its
	// original bytes).
	bad := filepath.Join(tmp, "pi", "bad")
	os.MkdirAll(filepath.Join(bad, "themes"), 0o755)
	badDst := filepath.Join(bad, "themes", "current.json")
	origBytes := []byte(`{"name":"unchanged"}`)
	os.WriteFile(badDst, origBytes, 0o644)
	// Read-only mode blocks CreateTemp inside themes/.
	os.Chmod(filepath.Join(bad, "themes"), 0o500)
	t.Cleanup(func() { os.Chmod(filepath.Join(bad, "themes"), 0o755) })

	origDirs := piProfileDirs
	piProfileDirs = func() []string { return []string{good, bad} }
	t.Cleanup(func() { piProfileDirs = origDirs })

	// hookPi may return the last write error but must proceed through
	// every profile. Good profile must be written; bad profile must
	// retain its original bytes.
	_ = hookPi(context.Background(), themeDir)

	goodBytes, err := os.ReadFile(filepath.Join(good, "themes", "current.json"))
	if err != nil {
		t.Fatalf("good profile write failed: %v", err)
	}
	if !containsField(goodBytes, `"name": "current"`) {
		t.Errorf("good profile did not get name=current: %s", goodBytes)
	}

	badBytes, err := os.ReadFile(badDst)
	if err != nil {
		t.Fatalf("bad profile file vanished: %v", err)
	}
	if string(badBytes) != string(origBytes) {
		t.Errorf("bad profile corrupted: got %q want %q", badBytes, origBytes)
	}
}

// TestHookPiNoOpWhenNoPayload covers the "graceful skip" behavior:
// theme with no derived/pi.json (and no legacy pi.json) is a no-op.
func TestHookPiNoOpWhenNoPayload(t *testing.T) {
	tmp := t.TempDir()
	themeDir := filepath.Join(tmp, "theme")
	os.MkdirAll(filepath.Join(themeDir, "derived"), 0o755)

	origDirs := piProfileDirs
	piProfileDirs = func() []string { return []string{filepath.Join(tmp, "pi", "agent")} }
	t.Cleanup(func() { piProfileDirs = origDirs })

	if err := hookPi(context.Background(), themeDir); err != nil {
		t.Errorf("no-payload hookPi should be no-op, got err: %v", err)
	}
}

// containsField does a naive substring check on marshaled JSON — good
// enough because MarshalIndent produces stable output for these test
// fixtures.
func containsField(b []byte, needle string) bool {
	return bytesIndex(b, []byte(needle)) >= 0
}

func bytesIndex(haystack, needle []byte) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
