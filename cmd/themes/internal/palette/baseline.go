package palette

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"text/template"
)

// baselineFS holds the 19 per-app baseline preambles that emitters
// write before the semantic block. Each file is the theme-independent
// scaffolding for one app.
//
// Convention:
//   internal/palette/baselines/<app>.tmpl
//
// Where <app> matches the Emitter.App field in emit.go / the new
// v4 emitter registry. Every baseline is a valid text/template — most
// are static content today (no placeholders), but the shape is
// reserved so emitters can inject structural values (typography size,
// effects.opacity, etc.) later without rewriting the pipeline.
//
//go:embed baselines/*.tmpl
var baselineFS embed.FS

// Baseline renders the baseline for the given app against theme t.
// A nil theme is legal — placeholder-less baselines return their
// static content unchanged.
//
// Missing baseline for an app returns ("", nil): callers treat that
// as "no baseline, jump straight to semantic block". This keeps the
// emitter pipeline uniform: an app can be added before its baseline
// is authored.
func Baseline(app string, t *Theme) (string, error) {
	name := "baselines/" + app + ".tmpl"
	data, err := fs.ReadFile(baselineFS, name)
	if err != nil {
		// Missing baseline is not a hard error.
		return "", nil
	}
	tmpl, err := template.New(app).Parse(string(data))
	if err != nil {
		return "", fmt.Errorf("parse baseline %s: %w", app, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, t); err != nil {
		return "", fmt.Errorf("execute baseline %s: %w", app, err)
	}
	return buf.String(), nil
}

// KnownBaselines returns the sorted list of every app that ships with
// a baseline. Used by tests and by the derive CLI's --debug flag.
func KnownBaselines() ([]string, error) {
	entries, err := fs.ReadDir(baselineFS, "baselines")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".tmpl") {
			continue
		}
		out = append(out, strings.TrimSuffix(name, ".tmpl"))
	}
	return out, nil
}
