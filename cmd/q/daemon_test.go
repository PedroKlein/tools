package main

import (
	"slices"
	"strings"
	"testing"
)

func TestBuildPiArgs_Basic(t *testing.T) {
	cfg := DefaultConfig()
	profile := Profile{
		Model: "claude-haiku-4.5",
	}

	args := BuildPiArgs(cfg, profile)

	// Must start with --mode rpc
	if args[0] != "--mode" || args[1] != "rpc" {
		t.Errorf("expected --mode rpc at start, got %v", args[:2])
	}

	// Must contain model
	assertContainsFlag(t, args, "--model", "claude-haiku-4.5")

	// Must contain BaseSystemPrompt
	assertContainsFlag(t, args, "--system-prompt", BaseSystemPrompt)

	// Must contain --no-session (always)
	assertContains(t, args, "--no-session")
}

func TestBuildPiArgs_AlwaysBasePrompt(t *testing.T) {
	cfg := DefaultConfig()
	// Profile with instruction — should NOT appear in args (instructions are per-request)
	profile := Profile{
		Instruction: "[some instruction]",
		Model:       "test",
	}

	args := BuildPiArgs(cfg, profile)

	// System prompt must be BaseSystemPrompt, not the profile instruction
	assertContainsFlag(t, args, "--system-prompt", BaseSystemPrompt)

	for _, a := range args {
		if a == "[some instruction]" {
			t.Error("profile instruction should not appear in pi args")
		}
	}
}

func TestBuildPiArgs_AlwaysNoSession(t *testing.T) {
	cfg := Config{
		PiFlags: []string{"--no-tools"},
	}
	profile := Profile{Model: "test"}

	args := BuildPiArgs(cfg, profile)
	assertContains(t, args, "--no-session")
}

func TestBuildPiArgs_NoSessionNotDuplicated(t *testing.T) {
	cfg := DefaultConfig()
	profile := Profile{Model: "test"}

	args := BuildPiArgs(cfg, profile)

	count := 0

	for _, a := range args {
		if a == "--no-session" {
			count++
		}
	}

	if count != 1 {
		t.Errorf("expected exactly 1 --no-session, got %d (args: %v)", count, args)
	}
}

func TestBuildPiArgs_ProfilePiFlags(t *testing.T) {
	cfg := DefaultConfig()
	profile := Profile{
		Model:   "claude-sonnet-4-20250514",
		PiFlags: []string{"--thinking", "low"},
	}

	args := BuildPiArgs(cfg, profile)
	assertContainsFlag(t, args, "--thinking", "low")
}

func TestBuildPiArgs_EmptyModel(t *testing.T) {
	cfg := Config{PiFlags: []string{"--no-session"}}
	profile := Profile{}

	args := BuildPiArgs(cfg, profile)
	for i, a := range args {
		if a == "--model" && i+1 < len(args) {
			t.Error("--model should not appear when model is empty")
		}
	}
}

func TestSocketPath(t *testing.T) {
	path := SocketPath()
	if !strings.HasSuffix(path, "q.sock") {
		t.Errorf("expected socket path ending in q.sock, got %s", path)
	}
}

func TestPIDPath(t *testing.T) {
	path := PIDPath()
	if !strings.HasSuffix(path, "q.pid") {
		t.Errorf("expected PID path ending in q.pid, got %s", path)
	}
}

func TestPiCommand_Default(t *testing.T) {
	t.Setenv("Q_PI_CMD", "")

	cmd := PiCommand()
	if cmd != "pi" {
		t.Errorf("expected 'pi', got %s", cmd)
	}
}

func TestPiCommand_Override(t *testing.T) {
	t.Setenv("Q_PI_CMD", "/usr/local/bin/my-pi")

	cmd := PiCommand()
	if cmd != "/usr/local/bin/my-pi" {
		t.Errorf("expected override, got %s", cmd)
	}
}

func TestNewDaemon(t *testing.T) {
	cfg := DefaultConfig()

	d := NewDaemon(cfg)
	if d.socketPath == "" {
		t.Error("expected non-empty socket path")
	}

	if d.pidPath == "" {
		t.Error("expected non-empty pid path")
	}
}

// --- Helpers ---

func assertContains(t *testing.T, args []string, want string) {
	t.Helper()

	if slices.Contains(args, want) {
		return
	}

	t.Errorf("expected %q in args %v", want, args)
}

func assertContainsFlag(t *testing.T, args []string, flag, value string) {
	t.Helper()

	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == value {
			return
		}
	}

	t.Errorf("expected %s %s in args", flag, value)
}
