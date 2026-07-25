package palette

import (
	"strings"
	"testing"
)

// TestBaselinesExistForAllExpectedApps verifies AC-1: 19 baseline files
// exist under internal/palette/baselines/.
func TestBaselinesExistForAllExpectedApps(t *testing.T) {
	got, err := KnownBaselines()
	if err != nil {
		t.Fatalf("KnownBaselines: %v", err)
	}
	// The full v4 emitter list. Order-insensitive comparison.
	want := []string{
		"aerospace", "bat", "btop", "delta", "fzf",
		"gh-dash", "ghostty", "k9s", "lazygit", "macos",
		"nvim", "opencode", "pi", "sketchybar", "starship",
		"television", "tmux", "wallpaper", "zsh-syntax-highlight",
	}
	if len(got) < 19 {
		t.Errorf("KnownBaselines returned %d, want >= 19: %v", len(got), got)
	}
	set := map[string]bool{}
	for _, a := range got {
		set[a] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("missing baseline %q", w)
		}
	}
}

// TestBaselineRendersWithNilTheme verifies AC-2: rendering a baseline
// against a null theme succeeds and returns the raw template content.
// This is the "safe scaffolding" contract — baselines never need theme
// data.
func TestBaselineRendersWithNilTheme(t *testing.T) {
	baselines, err := KnownBaselines()
	if err != nil {
		t.Fatalf("KnownBaselines: %v", err)
	}
	for _, app := range baselines {
		t.Run(app, func(t *testing.T) {
			out, err := Baseline(app, nil)
			if err != nil {
				t.Errorf("Baseline(%q, nil) err: %v", app, err)
			}
			if strings.TrimSpace(out) == "" {
				// wallpaper is documentation-only — comment lines allowed
				// but must still return non-empty content.
				t.Errorf("Baseline(%q, nil) returned empty", app)
			}
		})
	}
}

// TestBaselineForUnknownAppReturnsEmpty documents that adding an
// emitter before its baseline is authored is a warning, not a hard
// failure.
func TestBaselineForUnknownAppReturnsEmpty(t *testing.T) {
	out, err := Baseline("no-such-app", nil)
	if err != nil {
		t.Errorf("expected no error for missing baseline, got %v", err)
	}
	if out != "" {
		t.Errorf("expected empty string for missing baseline, got %q", out)
	}
}

// TestBaselineForTmuxHasSensibleDefaults verifies AC-2 concretely for
// tmux (the AC example): the baseline sets pane-border-style to a safe
// default before the semantic block overrides.
func TestBaselineForTmuxHasSensibleDefaults(t *testing.T) {
	out, err := Baseline("tmux", nil)
	if err != nil {
		t.Fatalf("Baseline(tmux): %v", err)
	}
	if !strings.Contains(out, "pane-border-style default") {
		t.Errorf("tmux baseline should set pane-border-style default; got:\n%s", out)
	}
}

// TestBaselineForSketchybarHasBarHeight verifies AC-2 concretely for
// sketchybar (the AC example): baseline exports BAR_HEIGHT=25.
func TestBaselineForSketchybarHasBarHeight(t *testing.T) {
	out, err := Baseline("sketchybar", nil)
	if err != nil {
		t.Fatalf("Baseline(sketchybar): %v", err)
	}
	if !strings.Contains(out, "BAR_HEIGHT=25") {
		t.Errorf("sketchybar baseline should set BAR_HEIGHT=25; got:\n%s", out)
	}
}
