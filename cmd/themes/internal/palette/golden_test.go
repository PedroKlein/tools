package palette

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// updateGoldens rewrites golden files when set. Enable with:
//
//	go test ./cmd/themes/internal/palette/ -run TestGoldenAllEmitters -update
var updateGoldens = flag.Bool("update", false, "rewrite golden emitter outputs under testdata/")

// TestGoldenAllEmitters runs every registered v4 emitter against the
// osaka-jade reference and compares against testdata/osaka-jade/derived/
// golden files.
//
// The wallpaper "emitter" is a resolver, not a file emit — it is
// therefore not in EmittersV4 and does not require a golden. The other
// 18 emitters do.
func TestGoldenAllEmitters(t *testing.T) {
	th, err := Load(filepath.Join("testdata", "osaka-jade"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	goldenDir := filepath.Join("testdata", "osaka-jade", "derived")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatalf("mkdir goldens: %v", err)
	}

	if len(EmittersV4) < 18 {
		t.Errorf("EmittersV4 has %d entries, want >= 18 (18 file emitters + wallpaper resolver = 19 conceptual apps)", len(EmittersV4))
	}

	for _, e := range EmittersV4 {
		e := e
		t.Run(e.App(), func(t *testing.T) {
			var buf bytes.Buffer
			if err := e.Emit(th, &buf); err != nil {
				t.Fatalf("Emit(%s): %v", e.App(), err)
			}
			got := buf.Bytes()
			goldenPath := filepath.Join(goldenDir, e.Filename())

			if *updateGoldens {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("wrote %s (%d bytes)", goldenPath, len(got))
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v (run with -update to regenerate)", goldenPath, err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("golden mismatch for %s\nrun 'go test ./cmd/themes/internal/palette/ -run TestGoldenAllEmitters -update' to regenerate\n--- want (%d bytes)\n%s\n--- got (%d bytes)\n%s",
					goldenPath, len(want), truncate(want, 200), len(got), truncate(got, 200))
			}
		})
	}
}

// truncate returns b or first n bytes with an ellipsis marker.
func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "\n... (truncated)"
}
