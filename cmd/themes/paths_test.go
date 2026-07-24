package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWriteFileAtomicPreservesPerms guards against silent perm loss on
// re-writes. Users may chmod theme files (e.g. 0755 for a sourceable
// snippet) and expect subsequent `themes derive` runs to keep the mode.
func TestWriteFileAtomicPreservesPerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o755 {
		t.Errorf("mode = %o, want 0755 (preserved from pre-existing file)", st.Mode().Perm())
	}
	data, _ := os.ReadFile(path)
	if string(data) != "v2" {
		t.Errorf("content = %q, want v2", data)
	}
}

// TestWriteFileAtomicNewFileUsesGivenMode: mode argument applies when
// destination does not exist yet.
func TestWriteFileAtomicNewFileUsesGivenMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new")
	if err := writeFileAtomic(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 0600 for new file", st.Mode().Perm())
	}
}

// TestWriteFileAtomicNoLeftoverTempOnSuccess ensures the tempfile is
// consumed by rename, not orphaned in the directory.
func TestWriteFileAtomicNoLeftoverTempOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	if err := writeFileAtomic(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("dir entries = %v, want just [target]", names)
	}
}
