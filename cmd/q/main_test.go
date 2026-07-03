package main

import "testing"

func TestRun_Help(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		code := run([]string{"q", arg})
		if code != 0 {
			t.Errorf("expected exit 0 for %q, got %d", arg, code)
		}
	}
}

func TestRun_NoArgs(t *testing.T) {
	code := run([]string{"q"})
	if code != 1 {
		t.Errorf("expected exit 1 with no args, got %d", code)
	}
}

func TestRun_Version(t *testing.T) {
	code := run([]string{"q", "--version"})
	if code != 0 {
		t.Errorf("expected exit 0 for --version, got %d", code)
	}
}
