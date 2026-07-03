package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Model != "claude-haiku-4.5" {
		t.Errorf("expected model claude-haiku-4.5, got %s", cfg.Model)
	}

	if cfg.DefaultProfile != "auto" {
		t.Errorf("expected defaultProfile auto, got %s", cfg.DefaultProfile)
	}

	if cfg.IdleTimeout != "15m" {
		t.Errorf("expected idleTimeout 15m, got %s", cfg.IdleTimeout)
	}

	if len(cfg.Profiles) != 5 {
		t.Errorf("expected 5 profiles, got %d", len(cfg.Profiles))
	}
	// Verify --no-session is always in piFlags
	if !slices.Contains(cfg.PiFlags, "--no-session") {
		t.Error("--no-session must be in default PiFlags")
	}
}

func TestLoadConfig_NoFile(t *testing.T) {
	t.Setenv("Q_CONFIG_PATH", "/nonexistent/path/config.json")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}

	if cfg.Model != "claude-haiku-4.5" {
		t.Errorf("expected defaults when file missing, got model %s", cfg.Model)
	}
}

func TestLoadConfig_ValidJSON(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.json")

	data := `{
		"model": "claude-sonnet-4-20250514",
		"idleTimeout": "30m",
		"defaultProfile": "explain",
		"profiles": {
			"custom": {
				"instruction": "[Be very helpful.]",
				"model": "claude-haiku-4.5"
			}
		}
	}`
	if err := os.WriteFile(cfgPath, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("Q_CONFIG_PATH", cfgPath)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model override, got %s", cfg.Model)
	}

	if cfg.IdleTimeout != "30m" {
		t.Errorf("expected timeout override, got %s", cfg.IdleTimeout)
	}

	if cfg.DefaultProfile != "explain" {
		t.Errorf("expected defaultProfile override, got %s", cfg.DefaultProfile)
	}
	// Custom profile added
	if p, ok := cfg.Profiles["custom"]; !ok {
		t.Error("expected custom profile to exist")
	} else if p.Instruction != "[Be very helpful.]" {
		t.Errorf("expected instruction, got %q", p.Instruction)
	}
	// Default profiles still present
	if _, ok := cfg.Profiles["cmd"]; !ok {
		t.Error("expected cmd profile to still exist")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()

	cfgPath := filepath.Join(tmp, "config.json")
	if err := os.WriteFile(cfgPath, []byte("not json{{{"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("Q_CONFIG_PATH", cfgPath)

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestResolveProfile_Explicit(t *testing.T) {
	cfg := DefaultConfig()

	p, err := ResolveProfile(cfg, "think")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected sonnet model for think profile, got %s", p.Model)
	}

	if len(p.PiFlags) == 0 || p.PiFlags[0] != "--thinking" {
		t.Error("expected --thinking flag in think profile")
	}
}

func TestResolveProfile_DefaultFallback(t *testing.T) {
	cfg := DefaultConfig()

	p, err := ResolveProfile(cfg, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// auto profile has no instruction (relies on base system prompt)
	if p.Instruction != "" {
		t.Errorf("expected empty instruction for auto profile, got %q", p.Instruction)
	}
}

func TestResolveProfile_Unknown(t *testing.T) {
	cfg := DefaultConfig()

	_, err := ResolveProfile(cfg, "nonexistent")
	if err == nil {
		t.Error("expected error for unknown profile")
	}
}

func TestResolveProfile_InheritsModel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Profiles["bare"] = Profile{}

	p, err := ResolveProfile(cfg, "bare")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Model != cfg.Model {
		t.Errorf("expected inherited model %s, got %s", cfg.Model, p.Model)
	}
}

func TestGetInstruction(t *testing.T) {
	cfg := DefaultConfig()

	// auto has no instruction
	if instr := GetInstruction(cfg, "auto"); instr != "" {
		t.Errorf("expected empty for auto, got %q", instr)
	}

	// cmd has instruction
	if instr := GetInstruction(cfg, "cmd"); instr == "" {
		t.Error("expected instruction for cmd profile")
	}

	// empty name uses default (auto)
	if instr := GetInstruction(cfg, ""); instr != "" {
		t.Errorf("expected empty for default (auto), got %q", instr)
	}
}

func TestIdleTimeoutDuration(t *testing.T) {
	cfg := DefaultConfig()

	d := cfg.IdleTimeoutDuration()
	if d != 15*time.Minute {
		t.Errorf("expected 15m, got %v", d)
	}

	cfg.IdleTimeout = "5m"

	d = cfg.IdleTimeoutDuration()
	if d != 5*time.Minute {
		t.Errorf("expected 5m, got %v", d)
	}

	cfg.IdleTimeout = "invalid"

	d = cfg.IdleTimeoutDuration()
	if d != 15*time.Minute {
		t.Errorf("expected fallback 15m for invalid, got %v", d)
	}
}

func TestExpandTilde(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{"~/foo/bar", filepath.Join(home, "foo/bar")},
		{"~", home},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		got := expandTilde(tt.input)
		if got != tt.want {
			t.Errorf("expandTilde(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBaseSystemPrompt(t *testing.T) {
	if BaseSystemPrompt == "" {
		t.Error("BaseSystemPrompt must not be empty")
	}

	if !containsStr(BaseSystemPrompt, "Never use markdown") {
		t.Error("BaseSystemPrompt should forbid markdown fences")
	}
}

func TestRunInstruction(t *testing.T) {
	if RunInstruction == "" {
		t.Error("RunInstruction must not be empty")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && contains(s, substr))
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}
