package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// makeThemeFixture writes a minimal but valid theme.json inside dir and
// returns the dir. Every emitter is fed by this file; the exact content
// doesn't matter for cache tests, only that it parses.
func makeThemeFixture(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Copy a real theme.json from the repo so the emitter set is exercised.
	src := filepath.Join(
		os.Getenv("HOME"),
		"dotfiles",
		"configs-shared",
		".config",
		"themes",
		"osaka-jade",
		"theme.json",
	)
	b, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("no reference theme.json at %s: %v", src, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "theme.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestDeriveCacheSkipsWhenStampMatches verifies AC-1:
// second call reports no changes when stamp + emitter outputs match.
func TestDeriveCacheSkipsWhenStampMatches(t *testing.T) {
	dir := makeThemeFixture(t, "cache-a")

	// First call: full derive, writes stamp.
	written1, err := deriveThemeV4(dir)
	if err != nil {
		t.Fatalf("first derive: %v", err)
	}
	if len(written1) == 0 {
		t.Fatal("first derive wrote nothing")
	}
	stampPath := filepath.Join(dir, "derived", stampFilename)
	if _, err := os.Stat(stampPath); err != nil {
		t.Fatalf(".stamp not written: %v", err)
	}

	// Second call: cache hit — should skip entirely, empty `written`.
	start := time.Now()
	written2, err := deriveThemeV4(dir)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("second derive: %v", err)
	}
	if len(written2) != 0 {
		t.Errorf("second derive re-wrote %d files, want 0 (cache hit)", len(written2))
	}
	if elapsed > 20*time.Millisecond {
		t.Errorf("cached derive took %v, want <20ms", elapsed)
	}
}

// TestDeriveCacheMissesOnThemeJSONChange verifies AC-2:
// touching theme.json invalidates the cache.
func TestDeriveCacheMissesOnThemeJSONChange(t *testing.T) {
	dir := makeThemeFixture(t, "cache-b")

	if _, err := deriveThemeV4(dir); err != nil {
		t.Fatalf("first derive: %v", err)
	}

	// Mutate theme.json — even a single byte flips the sha.
	tj := filepath.Join(dir, "theme.json")
	b, err := os.ReadFile(tj)
	if err != nil {
		t.Fatal(err)
	}
	// Append a byte so the JSON is still valid (trailing whitespace ok).
	if err := os.WriteFile(tj, append(b, ' '), 0o644); err != nil {
		t.Fatal(err)
	}

	written, err := deriveThemeV4(dir)
	if err != nil {
		t.Fatalf("second derive: %v", err)
	}
	if len(written) == 0 {
		t.Errorf("mutated theme.json should invalidate cache, got 0 writes")
	}
}

// TestDeriveForceBypassesStamp verifies AC-3:
// --force re-runs full derive even when the stamp matches.
func TestDeriveForceBypassesStamp(t *testing.T) {
	dir := makeThemeFixture(t, "cache-c")

	written1, err := deriveThemeV4(dir)
	if err != nil {
		t.Fatalf("first derive: %v", err)
	}

	written2, err := deriveThemeV4WithForce(dir, true)
	if err != nil {
		t.Fatalf("--force derive: %v", err)
	}
	if len(written2) != len(written1) {
		t.Errorf("--force wrote %d files, want %d (same as fresh derive)",
			len(written2), len(written1))
	}
}

// TestDeriveCacheMissesOnMissingOutput verifies the stamp-alone-is-not-
// enough invariant: even with a matching stamp, a missing emitter output
// must trigger a re-derive.
func TestDeriveCacheMissesOnMissingOutput(t *testing.T) {
	dir := makeThemeFixture(t, "cache-d")

	if _, err := deriveThemeV4(dir); err != nil {
		t.Fatal(err)
	}

	// Delete one derived file — stamp remains but the derived tree is
	// incomplete. Cache must not hit.
	files, err := os.ReadDir(filepath.Join(dir, "derived"))
	if err != nil {
		t.Fatal(err)
	}
	var victim string
	for _, f := range files {
		if f.Name() != stampFilename && !f.IsDir() {
			victim = filepath.Join(dir, "derived", f.Name())
			break
		}
	}
	if victim == "" {
		t.Fatal("could not find a derived file to delete")
	}
	if err := os.Remove(victim); err != nil {
		t.Fatal(err)
	}

	written, err := deriveThemeV4(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) == 0 {
		t.Errorf("missing output should invalidate cache, got 0 writes")
	}
}

// TestDeriveCacheTruncatedStampCounts AsMiss verifies AC constraint:
// tolerate a stale .stamp from an interrupted derive — treat as miss.
func TestDeriveCacheTruncatedStampCountsAsMiss(t *testing.T) {
	dir := makeThemeFixture(t, "cache-e")

	if _, err := deriveThemeV4(dir); err != nil {
		t.Fatal(err)
	}

	// Truncate the stamp to garbage.
	stampPath := filepath.Join(dir, "derived", stampFilename)
	if err := os.WriteFile(stampPath, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}

	written, err := deriveThemeV4(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) == 0 {
		t.Errorf("garbage stamp should count as cache miss, got 0 writes")
	}
	// And the miss should re-write the stamp correctly.
	if got := readStamp(dir); len(got) != 64 { // sha256 hex = 64 chars
		t.Errorf("stamp not re-written cleanly, got %q", got)
	}
}
