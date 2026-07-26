package reload

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestHookMacOSNoOpOnLinux verifies the Darwin gate: on non-Darwin,
// hookMacOS is a nil no-op regardless of sidecar contents.
func TestHookMacOSNoOpOnLinux(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("linux-only behavior test")
	}
	tmp := t.TempDir()
	themeDir := filepath.Join(tmp, "theme")
	os.MkdirAll(filepath.Join(themeDir, "derived"), 0o755)
	// Write a valid sidecar; hook must still no-op on non-Darwin.
	sc := macosSidecar{Mode: "dark", HighlightRGB: "0.5 0.5 0.5"}
	b, _ := json.Marshal(sc)
	os.WriteFile(filepath.Join(themeDir, "derived", "macos.json"), b, 0o644)

	if err := hookMacOS(context.Background(), themeDir); err != nil {
		t.Errorf("hookMacOS on non-Darwin must return nil, got: %v", err)
	}
}

// TestHookMacOSMissingSidecarIsNoOp verifies the graceful skip:
// theme without macos.json (or .macos.json legacy) is silent no-op
// even on Darwin.
func TestHookMacOSMissingSidecarIsNoOp(t *testing.T) {
	tmp := t.TempDir()
	themeDir := filepath.Join(tmp, "theme")
	os.MkdirAll(filepath.Join(themeDir, "derived"), 0o755)
	// No sidecar file written.
	if err := hookMacOS(context.Background(), themeDir); err != nil {
		t.Errorf("missing sidecar hookMacOS: %v", err)
	}
}

// TestHookMacOSRejectsCorruptSidecar verifies that a malformed JSON
// sidecar returns an error (not silent success).
func TestHookMacOSRejectsCorruptSidecar(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only path exercised")
	}
	tmp := t.TempDir()
	themeDir := filepath.Join(tmp, "theme")
	os.MkdirAll(filepath.Join(themeDir, "derived"), 0o755)
	os.WriteFile(filepath.Join(themeDir, "derived", "macos.json"), []byte("{ not json"), 0o644)

	if err := hookMacOS(context.Background(), themeDir); err == nil {
		t.Error("corrupt sidecar should surface parse error")
	}
}
