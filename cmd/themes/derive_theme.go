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
// Cache: after a successful derive, writes a .stamp file (sha256 of
// theme.json + overrides/) alongside the outputs. Subsequent calls
// short-circuit when the stamp matches AND every emitter output exists,
// returning an empty `written` list. Bypass with `themes derive --force`
// (deriveThemeV4Force below).
//
// Idempotent: given identical theme.json + overrides sidecars, produces
// byte-identical derived/*. That's the property the p1-9 AC checks.
func deriveThemeV4(themeDir string) (written []string, err error) {
	return deriveThemeV4WithForce(themeDir, false)
}

// deriveThemeV4WithForce is the underlying implementation exposed so
// runDerive can honor --force. force=true recomputes every emitter
// output and refreshes the stamp.
func deriveThemeV4WithForce(themeDir string, force bool) (written []string, err error) {
	if !force {
		if _, hit := deriveCacheHits(themeDir); hit {
			return nil, nil
		}
	}

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

	// Refresh stamp AFTER a successful full write. On partial write
	// (error mid-loop) the stamp is not touched, so the next derive
	// will retry from scratch — no stale-stamp wedging.
	stamp, err := computeThemeStamp(themeDir)
	if err != nil {
		return written, fmt.Errorf("stamp: %w", err)
	}
	if err := writeStamp(themeDir, stamp); err != nil {
		return written, fmt.Errorf("write stamp: %w", err)
	}
	return written, nil
}
