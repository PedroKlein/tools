package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

// runDerive implements `themes derive [<name>]`.
//
// Regenerates all 13 derived files for a theme from its alacritty.toml
// (mandatory) and palette.toml (optional). If <name> is omitted, uses the
// currently active theme (read from `.current` symlink).
//
// Ghostty preservation: if a theme dir has a hand-authored or upstream
// ghostty.conf (first non-empty line != "# primary"), it is skipped so
// upstream customizations survive `themes derive`.
func runDerive(args []string) {
	var name string
	switch len(args) {
	case 0:
		s, err := LoadState()
		if err != nil || s.Theme == "" {
			dieMsg("no active theme; usage: themes derive <name>", ExitError)
		}
		name = s.Theme
	case 1:
		name = args[0]
	default:
		dieMsg("usage: themes derive [<name>]", ExitError)
	}

	dir := themeDir(name)
	if !dirExists(dir) {
		dieMsg(fmt.Sprintf("theme not installed: %s", name), ExitNotFound)
	}

	written, skipped, err := deriveTheme(dir)
	if err != nil {
		dieMsg(err.Error(), ExitError)
	}
	if !jsonOutput {
		fmt.Fprintf(os.Stderr, "themes derive: %s regenerated (%d written, %d skipped)\n",
			name, len(written), len(skipped))
	}
}

// deriveTheme runs every emitter against the theme dir. Returns (written,
// skipped, error) — skipped is the list of files intentionally not
// touched (e.g. upstream ghostty.conf).
func deriveTheme(themeDir string) (written, skipped []string, err error) {
	p, err := palette.Load(themeDir)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range palette.Emitters {
		outPath := filepath.Join(themeDir, e.Filename)

		// Preserve upstream ghostty.conf: our derive always writes
		// `# primary` as the first non-empty line. Any other first line
		// means the file is hand-authored (or upstream from Omarchy)
		// and must survive `themes derive`.
		if e.Filename == "ghostty.conf" && fileExists(outPath) {
			if first := firstNonEmptyLine(outPath); first != "" && first != "# primary" {
				skipped = append(skipped, e.Filename)
				continue
			}
		}

		// Emit to a memory buffer, then atomically write. Prevents
		// crash/disk-full during Emit() from truncating a live app config.
		var buf bytes.Buffer
		if err := e.Emit(&buf, p); err != nil {
			return written, skipped, fmt.Errorf("emit %s: %w", e.App, err)
		}
		if err := writeFileAtomic(outPath, buf.Bytes(), 0o644); err != nil {
			return written, skipped, fmt.Errorf("write %s: %w", outPath, err)
		}
		written = append(written, e.Filename)
	}
	return written, skipped, nil
}

// firstNonEmptyLine returns the first non-blank, non-whitespace line in
// path, or "" on read error / empty file.
func firstNonEmptyLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, raw := range splitLines(string(data)) {
		s := trimSpace(raw)
		if s != "" {
			return s
		}
	}
	return ""
}

// splitLines is a tiny helper matching strings.Split(s, "\n") without
// pulling the whole strings package into this file's imports.
func splitLines(s string) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// trimSpace strips ASCII whitespace from both ends of s.
func trimSpace(s string) string {
	l, r := 0, len(s)
	for l < r && (s[l] == ' ' || s[l] == '\t' || s[l] == '\r') {
		l++
	}
	for r > l && (s[r-1] == ' ' || s[r-1] == '\t' || s[r-1] == '\r') {
		r--
	}
	return s[l:r]
}
