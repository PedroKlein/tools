package reload

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

// TestBuildOSCBlobStandardKeys verifies the AC: emits OSC 10/11/12 +
// OSC 4;N (0..15) + OSC 1337 SetColors= (iTerm2 proprietary).
func TestBuildOSCBlobStandardKeys(t *testing.T) {
	th := &palette.Theme{}
	th.Palette.Semantic.Fg = "#ff0000"
	th.Palette.Semantic.Bg = "#00ff00"
	th.Palette.Semantic.Cursor = "#0000ff"
	// Fill the 16 ansi slots with a distinguishable pattern.
	for i := range 16 {
		th.Palette.Ansi[i] = "#010203" // arbitrary but non-empty
	}

	got := string(buildOSCBlob(th))

	// Must contain OSC 10 with fg.
	if !strings.Contains(got, "\x1b]10;#ff0000\x07") {
		t.Errorf("blob missing OSC 10 fg frame")
	}
	// OSC 11 with bg.
	if !strings.Contains(got, "\x1b]11;#00ff00\x07") {
		t.Errorf("blob missing OSC 11 bg frame")
	}
	// OSC 12 with cursor.
	if !strings.Contains(got, "\x1b]12;#0000ff\x07") {
		t.Errorf("blob missing OSC 12 cursor frame")
	}
	// OSC 4;0..15 — check the boundaries.
	if !strings.Contains(got, "\x1b]4;0;#010203\x07") {
		t.Errorf("blob missing OSC 4;0 frame")
	}
	if !strings.Contains(got, "\x1b]4;15;#010203\x07") {
		t.Errorf("blob missing OSC 4;15 frame")
	}
	// OSC 1337 SetColors=<payload>.
	if !strings.Contains(got, "\x1b]1337;SetColors=") {
		t.Errorf("blob missing OSC 1337 SetColors frame")
	}
}

func TestBuildOSCBlobWithoutBackground(t *testing.T) {
	th := &palette.Theme{}
	th.Palette.Semantic.Fg = "#ff0000"
	th.Palette.Semantic.Bg = "#00ff00"
	th.Palette.Semantic.Cursor = "#0000ff"
	for i := range 16 {
		th.Palette.Ansi[i] = "#010203"
	}

	got := string(buildOSCBlobWithOptions(th, oscOptions{IncludeBackground: false}))

	if strings.Contains(got, "\x1b]11;") {
		t.Errorf("blob emitted OSC 11 background while IncludeBackground=false")
	}
	if strings.Contains(got, "bg=00ff00") {
		t.Errorf("blob emitted SetColors bg while IncludeBackground=false")
	}
	if !strings.Contains(got, "\x1b]10;#ff0000\x07") {
		t.Errorf("blob missing OSC 10 fg frame")
	}
	if !strings.Contains(got, "\x1b]4;15;#010203\x07") {
		t.Errorf("blob missing OSC 4;15 frame")
	}
}

func TestBuildOSCBlobBroadcastOmitsBackground(t *testing.T) {
	th := &palette.Theme{}
	th.Palette.Semantic.Fg = "#ff0000"
	th.Palette.Semantic.Bg = "#00ff00"
	th.Palette.Semantic.Cursor = "#0000ff"
	for i := range 16 {
		th.Palette.Ansi[i] = "#010203"
	}

	got := string(buildOSCBlobWithOptions(th, oscOptions{IncludeBackground: false}))
	if strings.Contains(got, "\x1b]11;") || strings.Contains(got, "bg=00ff00") {
		t.Fatalf("broadcast OSC emitted background: %q", got)
	}
}

func TestFilterTTYsSkipsTmuxClientTTYs(t *testing.T) {
	ttys := []string{"/dev/ttys001", "/dev/ttys002", "/dev/ttys003"}
	skip := map[string]struct{}{"/dev/ttys002": {}}
	got := filterTTYs(ttys, skip)
	want := []string{"/dev/ttys001", "/dev/ttys003"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("filterTTYs = %v, want %v", got, want)
	}
}

func TestCommitOSCTargetsSendsBackgroundToTmuxClientsOnly(t *testing.T) {
	direct := []string{"/dev/ttys001"}
	clients := map[string]struct{}{"/dev/ttys002": {}}
	panes := []string{"/dev/ttys003"}

	full, noBackground := commitOSCTargets(direct, clients, panes)
	if got, want := strings.Join(full, ","), "/dev/ttys001,/dev/ttys002"; got != want {
		t.Fatalf("full-background targets = %q, want %q", got, want)
	}
	if got, want := strings.Join(noBackground, ","), "/dev/ttys003"; got != want {
		t.Fatalf("no-background targets = %q, want %q", got, want)
	}
}

func TestHookOSCPreviewInsideTmuxWritesBackgroundToClientTTY(t *testing.T) {
	tmp := t.TempDir()
	paneTTY := filepath.Join(tmp, "pane-tty")
	clientTTY := filepath.Join(tmp, "client-tty")
	if err := os.WriteFile(paneTTY, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clientTTY, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	origTTYPath := controllingTTYPath
	controllingTTYPath = paneTTY
	t.Cleanup(func() { controllingTTYPath = origTTYPath })
	origClientTTY := currentTmuxClientTTY
	currentTmuxClientTTY = func(context.Context) string { return clientTTY }
	t.Cleanup(func() { currentTmuxClientTTY = origClientTTY })
	t.Setenv("TMUX", "/tmp/themes-test-tmux,123,0")

	ctx := context.WithValue(context.Background(), liveApplyContextKey{}, true)
	themeDir := filepath.Join("..", "palette", "testdata", "osaka-jade")
	if err := hookOSC(ctx, themeDir); err != nil {
		t.Fatalf("hookOSC returned error: %v", err)
	}

	paneBytes, err := os.ReadFile(paneTTY)
	if err != nil {
		t.Fatal(err)
	}
	clientBytes, err := os.ReadFile(clientTTY)
	if err != nil {
		t.Fatal(err)
	}
	pane := string(paneBytes)
	client := string(clientBytes)
	if strings.Contains(pane, "\x1b]11;") || strings.Contains(pane, "bg=111C18") {
		t.Fatalf("tmux pane tty received background OSC: %q", pane)
	}
	if !strings.Contains(client, "\x1b]11;#111C18\x07") || !strings.Contains(client, "bg=111C18") {
		t.Fatalf("tmux client tty did not receive background OSC: %q", client)
	}
}

func TestHookOSCPreviewWritesControllingTTYWithoutBackgroundInTmux(t *testing.T) {
	ttyPath := filepath.Join(t.TempDir(), "tty")
	if err := os.WriteFile(ttyPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	origTTYPath := controllingTTYPath
	controllingTTYPath = ttyPath
	t.Cleanup(func() { controllingTTYPath = origTTYPath })
	t.Setenv("TMUX", "/tmp/themes-test-tmux,123,0")

	ctx := context.WithValue(context.Background(), liveApplyContextKey{}, true)
	themeDir := filepath.Join("..", "palette", "testdata", "osaka-jade")
	if err := hookOSC(ctx, themeDir); err != nil {
		t.Fatalf("hookOSC returned error: %v", err)
	}

	gotBytes, err := os.ReadFile(ttyPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(gotBytes)
	if !strings.Contains(got, "\x1b]10;#C1C497\x07") {
		t.Fatalf("preview did not write foreground OSC to controlling tty: %q", got)
	}
	if strings.Contains(got, "\x1b]11;") || strings.Contains(got, "bg=111C18") {
		t.Fatalf("preview inside tmux emitted background OSC: %q", got)
	}
}

// TestBuildOSCBlobSetColorsFormat verifies the iTerm2 payload contract:
// key=value pairs comma-separated, hex WITHOUT leading '#'.
func TestBuildOSCBlobSetColorsFormat(t *testing.T) {
	th := &palette.Theme{}
	th.Palette.Semantic.Fg = "#ff0000"
	th.Palette.Semantic.Bg = "#00ff00"
	th.Palette.Semantic.Cursor = "" // omitted → should fall back to fg
	for i := range 16 {
		th.Palette.Ansi[i] = "#abcdef"
	}

	got := string(buildOSCBlob(th))

	// Extract the SetColors payload.
	start := strings.Index(got, "SetColors=")
	if start < 0 {
		t.Fatal("no SetColors frame")
	}
	end := strings.Index(got[start:], "\x07")
	if end < 0 {
		t.Fatal("SetColors frame not BEL-terminated")
	}
	payload := got[start+len("SetColors=") : start+end]

	// Payload MUST NOT contain '#'.
	if strings.Contains(payload, "#") {
		t.Errorf("SetColors payload contains '#': %q", payload)
	}
	// Must include fg / bg / cursor / ansiN entries.
	for _, want := range []string{"fg=ff0000", "bg=00ff00", "cursor=ff0000", "ansi0=abcdef", "ansi15=abcdef"} {
		if !strings.Contains(payload, want) {
			t.Errorf("SetColors payload missing %q: %q", want, payload)
		}
	}
}

// TestBuildOSCBlobEmptyThemeIsNoBytes verifies that an empty theme
// produces at most a SetColors frame with empty values — but we
// explicitly want NO output for a nil theme so callers can bail early.
func TestBuildOSCBlobEmptyThemeIsNoBytes(t *testing.T) {
	if got := buildOSCBlob(nil); len(got) != 0 {
		t.Errorf("nil theme produced %d bytes, want 0", len(got))
	}
}

// TestBuildOSCBlobCursorFallsBackToFg covers the semantic default:
// when cursor is unset, OSC 12 emits the foreground color.
func TestBuildOSCBlobCursorFallsBackToFg(t *testing.T) {
	th := &palette.Theme{}
	th.Palette.Semantic.Fg = "#deadbe"
	th.Palette.Semantic.Bg = "#000000"
	// Cursor deliberately unset.

	got := string(buildOSCBlob(th))
	if !strings.Contains(got, "\x1b]12;#deadbe\x07") {
		t.Errorf("cursor fallback missing: OSC 12 should carry fg. Got: %q", got)
	}
}

// Benchmark for the AC's <50ms-on-6-panes target. Blob is a fixed
// sequence for a given theme; discovery + write is where wallclock is
// spent (both mocked out here). This bench proves the pure-Go blob
// build is not a bottleneck.
func BenchmarkBuildOSCBlob(b *testing.B) {
	th := &palette.Theme{}
	th.Palette.Semantic.Fg = "#ff0000"
	th.Palette.Semantic.Bg = "#00ff00"
	th.Palette.Semantic.Cursor = "#0000ff"
	for i := 0; i < 16; i++ {
		th.Palette.Ansi[i] = "#010203"
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buildOSCBlob(th)
	}
}
