package reload

import (
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
	for i := 0; i < 16; i++ {
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

// TestBuildOSCBlobSetColorsFormat verifies the iTerm2 payload contract:
// key=value pairs comma-separated, hex WITHOUT leading '#'.
func TestBuildOSCBlobSetColorsFormat(t *testing.T) {
	th := &palette.Theme{}
	th.Palette.Semantic.Fg = "#ff0000"
	th.Palette.Semantic.Bg = "#00ff00"
	th.Palette.Semantic.Cursor = "" // omitted → should fall back to fg
	for i := 0; i < 16; i++ {
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
