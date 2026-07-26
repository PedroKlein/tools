package reload

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// TestHookMacOSNoOpOnLinux verifies the Darwin gate: on non-Darwin,
// hookMacOS is a nil no-op regardless of sidecar contents.
func TestHookMacOSNoOpOnLinux(t *testing.T) {
	origGoos := goos
	goos = "linux"
	t.Cleanup(func() { goos = origGoos })
	tmp := t.TempDir()
	themeDir := filepath.Join(tmp, "theme")
	if err := os.MkdirAll(filepath.Join(themeDir, "derived"), 0o750); err != nil {
		t.Fatal(err)
	}
	sc := macosSidecar{Mode: "dark", HighlightRGB: "0.5 0.5 0.5"}
	b, err := json.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "derived", "macos.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}

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
	if err := os.MkdirAll(filepath.Join(themeDir, "derived"), 0o750); err != nil {
		t.Fatal(err)
	}
	// No sidecar file written.
	if err := hookMacOS(context.Background(), themeDir); err != nil {
		t.Errorf("missing sidecar hookMacOS: %v", err)
	}
}

// TestHookMacOSRejectsCorruptSidecar verifies that a malformed JSON
// sidecar returns an error (not silent success).
func TestHookMacOSRejectsCorruptSidecar(t *testing.T) {
	origGoos := goos
	goos = "darwin"
	t.Cleanup(func() { goos = origGoos })
	tmp := t.TempDir()
	themeDir := filepath.Join(tmp, "theme")
	if err := os.MkdirAll(filepath.Join(themeDir, "derived"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "derived", "macos.json"), []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := hookMacOS(context.Background(), themeDir); err == nil {
		t.Error("corrupt sidecar should surface parse error")
	}
}

func TestHookMacOSRestartsCachedUI(t *testing.T) {
	origGoos := goos
	goos = "darwin"
	t.Cleanup(func() { goos = origGoos })

	origLookPath := macosLookPath
	macosLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() { macosLookPath = origLookPath })

	origRun := macosRun
	var killall []string
	macosRun = func(_ context.Context, name string, args ...string) error {
		if name == "killall" && len(args) == 1 {
			killall = append(killall, args[0])
		}
		return nil
	}
	t.Cleanup(func() { macosRun = origRun })

	tmp := t.TempDir()
	themeDir := filepath.Join(tmp, "theme")
	if err := os.MkdirAll(filepath.Join(themeDir, "derived"), 0o750); err != nil {
		t.Fatal(err)
	}
	accent := 3
	sc := macosSidecar{Mode: "dark", AccentInt: &accent, HighlightRGB: "0.4 0.5 0.6"}
	b, err := json.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "derived", "macos.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := hookMacOS(context.Background(), themeDir); err != nil {
		t.Fatalf("hookMacOS: %v", err)
	}
	want := []string{"cfprefsd", "Dock", "SystemUIServer"}
	if !reflect.DeepEqual(killall, want) {
		t.Fatalf("killall sequence = %v, want %v", killall, want)
	}
}

func TestRunMacOSKillallUsesShortTimeout(t *testing.T) {
	origRun := macosRun
	var hasDeadline bool
	var maxRemaining time.Duration
	var got []string
	macosRun = func(ctx context.Context, name string, args ...string) error {
		got = append([]string{name}, args...)
		deadline, ok := ctx.Deadline()
		hasDeadline = ok
		maxRemaining = time.Until(deadline)
		return nil
	}
	t.Cleanup(func() { macosRun = origRun })

	runMacOSKillall(context.Background(), "Dock")
	if !reflect.DeepEqual(got, []string{"killall", "Dock"}) {
		t.Fatalf("command = %v, want [killall Dock]", got)
	}
	if !hasDeadline {
		t.Fatal("killall context has no deadline")
	}
	if maxRemaining > 1500*time.Millisecond || maxRemaining <= 0 {
		t.Fatalf("killall timeout remaining = %s, want within 1.5s", maxRemaining)
	}
}
