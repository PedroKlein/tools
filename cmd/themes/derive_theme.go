package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

// deriveThemeV4 runs every v4 emitter against <themeDir>/theme.json and
// writes outputs into <themeDir>/derived/. Returns the list of written
// filenames (relative to derived/).
//
// Idempotent: given identical theme.json + overrides sidecars, produces
// byte-identical derived/*. That's the property the p1-9 AC checks.
func deriveThemeV4(themeDir string) (written []string, err error) {
	t, err := palette.Load(themeDir)
	if err != nil {
		return nil, fmt.Errorf("load theme.json: %w", err)
	}
	outDir := filepath.Join(themeDir, "derived")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	for _, e := range palette.EmittersV4 {
		var buf bytes.Buffer
		if err := e.Emit(t, &buf); err != nil {
			return written, fmt.Errorf("emit %s: %w", e.App(), err)
		}
		outPath := filepath.Join(outDir, e.Filename())
		if err := writeFileAtomic(outPath, buf.Bytes(), 0o644); err != nil {
			return written, fmt.Errorf("write %s: %w", outPath, err)
		}
		written = append(written, e.Filename())
	}
	return written, nil
}
