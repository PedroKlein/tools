package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

// runDerive implements `themes derive [<name>] [--force] [--check-all]`.
//
// Regenerates every derived file under <themeDir>/derived/ from
// <themeDir>/theme.json. Skips the emit pipeline when a matching
// .stamp exists AND every emitter output is present; use --force to
// bypass the cache.
//
// --check-all iterates every theme under themesRoot(), verifies each
// theme's committed derived/ matches what a fresh derive would produce
// (byte-diff), and exits non-zero on any drift. Intended for CI or a
// pre-commit hook once shipped themes' derived/ is tracked.
//
// Themes without theme.json (unmigrated v3-format bundles) fail with a
// pointer at /theme-import. If <name> is omitted, uses the currently
// active theme.
func runDerive(args []string) {
	force := false
	checkAll := false
	var positional []string
	for _, a := range args {
		switch a {
		case "--force", "-f":
			force = true
		case "--check-all":
			checkAll = true
		default:
			positional = append(positional, a)
		}
	}

	if checkAll {
		runDeriveCheckAll()
		return
	}

	var name string
	switch len(positional) {
	case 0:
		s := activeState()
		if s.Theme == "" {
			dieMsg("no active theme; usage: themes derive <name> [--force]", ExitError)
		}
		name = s.Theme
	case 1:
		name = positional[0]
	default:
		dieMsg("usage: themes derive [<name>] [--force | --check-all]", ExitError)
	}

	dir := themeDir(name)
	if !dirExists(dir) {
		dieMsg(fmt.Sprintf("theme not installed: %s", name), ExitNotFound)
	}
	if !fileExists(filepath.Join(dir, "theme.json")) {
		dieMsg(fmt.Sprintf(
			"theme %s has no theme.json (v3 format); run /theme-import to migrate", name),
			ExitError)
	}

	written, err := deriveThemeV4WithForce(dir, force)
	if err != nil {
		dieMsg(err.Error(), ExitError)
	}
	if !jsonOutput {
		if len(written) == 0 {
			fmt.Fprintf(os.Stderr, "themes derive: %s no changes, skipping (stamp match)\n", name)
		} else {
			fmt.Fprintf(os.Stderr, "themes derive: %s regenerated (%d files → derived/)\n", name, len(written))
		}
	}
}

// runDeriveCheckAll walks every theme dir under themesRoot(), regenerates
// its derived files in a scratch buffer, and byte-compares against the
// on-disk derived/. Any mismatch is a drift; the run exits non-zero and
// lists every offending file.
//
// Intended for CI once shipped themes have their derived/ tracked in git.
// Today's `.gitignore` still excludes derived/, so drift-detection here
// warns about un-committed-derived-tree themes rather than tracked-drift;
// still useful as a smoke.
func runDeriveCheckAll() {
	root := themesRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		dieMsg(fmt.Sprintf("themes root not readable: %v", err), ExitError)
	}
	var drifts []driftEntry
	var checked int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) > 0 && name[0] == '.' {
			continue // .hooks/, .bin/ etc
		}
		dir := themeDir(name)
		if !fileExists(filepath.Join(dir, "theme.json")) {
			continue // not a valid theme dir
		}
		checked++
		drifted := checkThemeDrift(dir)
		for _, d := range drifted {
			drifts = append(drifts, driftEntry{Theme: name, File: d.file, Reason: d.reason})
		}
	}
	if !jsonOutput {
		if len(drifts) == 0 {
			fmt.Fprintf(os.Stderr, "themes derive --check-all: %d themes clean\n", checked)
			return
		}
		fmt.Fprintf(os.Stderr, "themes derive --check-all: drift in %d themes / %d files:\n",
			uniqueThemes(drifts), len(drifts))
		for _, d := range drifts {
			fmt.Fprintf(os.Stderr, "  %s/%s: %s\n", d.Theme, d.File, d.Reason)
		}
	}
	if len(drifts) > 0 {
		os.Exit(ExitError)
	}
}

type driftFinding struct {
	file   string
	reason string
}

// checkThemeDrift regenerates emitters in-memory and compares against
// on-disk derived/ files. Missing outputs, byte-mismatched outputs,
// and load errors all yield drift entries.
func checkThemeDrift(themeDir string) []driftFinding {
	var out []driftFinding
	t, err := palette.Load(themeDir)
	if err != nil {
		return []driftFinding{{file: "theme.json", reason: fmt.Sprintf("load: %v", err)}}
	}
	derived := filepath.Join(themeDir, "derived")
	for _, e := range palette.EmittersV4 {
		var buf bytes.Buffer
		if err := e.Emit(t, &buf); err != nil {
			out = append(out, driftFinding{file: e.Filename(), reason: fmt.Sprintf("emit: %v", err)})
			continue
		}
		onDisk, err := os.ReadFile(filepath.Join(derived, e.Filename()))
		if err != nil {
			out = append(out, driftFinding{file: e.Filename(), reason: "missing on disk"})
			continue
		}
		if !bytes.Equal(onDisk, buf.Bytes()) {
			out = append(out, driftFinding{file: e.Filename(),
				reason: fmt.Sprintf("byte-differs (on-disk %d, freshly-emitted %d)", len(onDisk), buf.Len())})
		}
	}
	return out
}

func uniqueThemes(drifts []driftEntry) int {
	seen := map[string]bool{}
	for _, d := range drifts {
		seen[d.Theme] = true
	}
	return len(seen)
}

type driftEntry struct {
	Theme  string
	File   string
	Reason string
}
