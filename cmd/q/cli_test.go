package main

import "testing"

func TestParseArgs_Profile(t *testing.T) {
	p := ParseArgs([]string{"-p", "think", "explain", "monads"})
	if p.Profile != "think" {
		t.Errorf("expected profile 'think', got %q", p.Profile)
	}

	if p.Prompt != "explain monads" {
		t.Errorf("expected prompt 'explain monads', got %q", p.Prompt)
	}
}

func TestParseArgs_LongProfile(t *testing.T) {
	p := ParseArgs([]string{"--profile", "code", "write", "hello world"})
	if p.Profile != "code" {
		t.Errorf("expected profile 'code', got %q", p.Profile)
	}

	if p.Prompt != "write hello world" {
		t.Errorf("expected prompt, got %q", p.Prompt)
	}
}

func TestParseArgs_Copy(t *testing.T) {
	p := ParseArgs([]string{"-c", "tar syntax"})
	if !p.Copy {
		t.Error("expected Copy=true")
	}

	if p.Prompt != "tar syntax" {
		t.Errorf("expected prompt 'tar syntax', got %q", p.Prompt)
	}
}

func TestParseArgs_Run(t *testing.T) {
	p := ParseArgs([]string{"--run", "delete DS_Store"})
	if !p.Run {
		t.Error("expected Run=true")
	}

	if p.Prompt != "delete DS_Store" {
		t.Errorf("expected prompt, got %q", p.Prompt)
	}
}

func TestParseArgs_RunShort(t *testing.T) {
	p := ParseArgs([]string{"-r", "list ports"})
	if !p.Run {
		t.Error("expected Run=true with -r")
	}
}

func TestParseArgs_New(t *testing.T) {
	p := ParseArgs([]string{"--new", "different topic"})
	if !p.New {
		t.Error("expected New=true")
	}

	if p.Prompt != "different topic" {
		t.Errorf("expected prompt, got %q", p.Prompt)
	}
}

func TestParseArgs_AllFlags(t *testing.T) {
	p := ParseArgs([]string{"-p", "cmd", "-c", "--new", "--run", "do something"})
	if p.Profile != "cmd" {
		t.Errorf("expected profile 'cmd', got %q", p.Profile)
	}

	if !p.Copy {
		t.Error("expected Copy=true")
	}

	if !p.New {
		t.Error("expected New=true")
	}

	if !p.Run {
		t.Error("expected Run=true")
	}

	if p.Prompt != "do something" {
		t.Errorf("expected prompt 'do something', got %q", p.Prompt)
	}
}

func TestParseArgs_Subcommands(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"start"}, "start"},
		{[]string{"stop"}, "stop"},
		{[]string{"status"}, "status"},
		{[]string{"profiles"}, "profiles"},
	}
	for _, tt := range tests {
		p := ParseArgs(tt.args)
		if p.Subcommand != tt.want {
			t.Errorf("ParseArgs(%v): expected subcommand %q, got %q", tt.args, tt.want, p.Subcommand)
		}
	}
}

func TestParseArgs_ProfilesNames(t *testing.T) {
	p := ParseArgs([]string{"profiles", "--names"})
	if p.Subcommand != "profiles" {
		t.Errorf("expected subcommand 'profiles', got %q", p.Subcommand)
	}

	if !p.NamesOnly {
		t.Error("expected NamesOnly=true")
	}
}

func TestParseArgs_PromptOnly(t *testing.T) {
	p := ParseArgs([]string{"how", "do", "I", "list", "files"})
	if p.Subcommand != "" {
		t.Errorf("expected no subcommand, got %q", p.Subcommand)
	}

	if p.Prompt != "how do I list files" {
		t.Errorf("expected joined prompt, got %q", p.Prompt)
	}
}

func TestParseArgs_Empty(t *testing.T) {
	p := ParseArgs([]string{})
	if p.Prompt != "" {
		t.Errorf("expected empty prompt, got %q", p.Prompt)
	}
}
