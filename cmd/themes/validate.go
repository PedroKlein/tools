package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Embed the schema so `themes validate` works regardless of install path.
// Sibling of cmd_validate.go: cmd/themes/schema/theme.schema.json.
//
//go:embed schema/theme.schema.json
var schemaFS embed.FS

const schemaEmbedPath = "schema/theme.schema.json"

// runValidate implements `themes validate <name-or-path>`.
//
// Loads the target theme.json (either <name> resolved against themesRoot()
// or a direct path to a .json file / theme directory) and validates it
// against the embedded v1 JSON Schema. Prints one line per violation,
// each anchored by JSON Pointer path.
//
// Exit codes:
//   0            valid
//   ExitError    invalid, unreadable, or malformed
//   ExitNotFound theme not installed / path missing
func runValidate(args []string) {
	if len(args) != 1 {
		dieMsg("usage: themes validate <name-or-path>", ExitError)
	}

	themePath, err := resolveThemeJSONPath(args[0])
	if err != nil {
		var nf notFoundError
		if errors.As(err, &nf) {
			dieMsg(err.Error(), ExitNotFound)
		}
		dieMsg(err.Error(), ExitError)
	}

	raw, err := os.ReadFile(themePath)
	if err != nil {
		dieMsg(fmt.Sprintf("read %s: %v", themePath, err), ExitError)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		dieMsg(fmt.Sprintf("%s is not valid JSON: %v", themePath, err), ExitError)
	}

	schema, err := loadEmbeddedSchema()
	if err != nil {
		dieMsg(fmt.Sprintf("load schema: %v", err), ExitError)
	}

	if err := schema.Validate(doc); err != nil {
		violations := flattenValidationError(err)
		if jsonOutput {
			out := struct {
				Path       string   `json:"path"`
				Valid      bool     `json:"valid"`
				Violations []string `json:"violations"`
			}{Path: themePath, Valid: false, Violations: violations}
			writeJSON(out)
			os.Exit(ExitError)
		}
		fmt.Fprintf(os.Stderr, "themes: %s failed validation:\n", themePath)
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, "  - %s\n", v)
		}
		os.Exit(ExitError)
	}

	if jsonOutput {
		out := struct {
			Path  string `json:"path"`
			Valid bool   `json:"valid"`
		}{Path: themePath, Valid: true}
		writeJSON(out)
		return
	}
	fmt.Fprintf(os.Stderr, "themes: %s is valid\n", themePath)
}

// notFoundError distinguishes ExitNotFound from ExitError.
type notFoundError struct{ path string }

func (e notFoundError) Error() string {
	return fmt.Sprintf("theme not found: %s", e.path)
}

// resolveThemeJSONPath accepts:
//   - a bare theme name  → <themesRoot>/<name>/theme.json
//   - a directory path   → <dir>/theme.json
//   - a .json file path  → used as-is
func resolveThemeJSONPath(arg string) (string, error) {
	// Direct .json file
	if strings.HasSuffix(arg, ".json") {
		if _, err := os.Stat(arg); err != nil {
			return "", notFoundError{arg}
		}
		abs, err := filepath.Abs(arg)
		if err != nil {
			return "", err
		}
		return abs, nil
	}

	// Directory path (contains a separator, or exists as a dir on disk)
	if strings.ContainsRune(arg, filepath.Separator) {
		st, err := os.Stat(arg)
		if err != nil || !st.IsDir() {
			return "", notFoundError{arg}
		}
		p := filepath.Join(arg, "theme.json")
		if _, err := os.Stat(p); err != nil {
			return "", notFoundError{p}
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		return abs, nil
	}

	// Bare theme name → themesRoot()/<name>/theme.json
	dir := themeDir(arg)
	if !dirExists(dir) {
		return "", notFoundError{dir}
	}
	p := filepath.Join(dir, "theme.json")
	if _, err := os.Stat(p); err != nil {
		return "", notFoundError{p}
	}
	return p, nil
}

// loadEmbeddedSchema compiles the JSON Schema shipped with the binary.
func loadEmbeddedSchema() (*jsonschema.Schema, error) {
	data, err := fs.ReadFile(schemaFS, schemaEmbedPath)
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020
	// Register under a synthetic URL because the compiler wants one.
	if err := compiler.AddResource(schemaEmbedPath, strings.NewReader(string(data))); err != nil {
		return nil, err
	}
	return compiler.Compile(schemaEmbedPath)
}

// flattenValidationError turns a jsonschema.ValidationError tree into a
// flat, human-friendly list of "instance path: message" strings. Only
// leaf failures are surfaced (branch nodes are just structural).
func flattenValidationError(err error) []string {
	var out []string
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []string{err.Error()}
	}
	var walk func(v *jsonschema.ValidationError)
	walk = func(v *jsonschema.ValidationError) {
		if len(v.Causes) == 0 {
			path := v.InstanceLocation
			if path == "" {
				path = "/"
			}
			out = append(out, fmt.Sprintf("%s: %s", path, v.Message))
			return
		}
		for _, c := range v.Causes {
			walk(c)
		}
	}
	walk(ve)
	if len(out) == 0 {
		out = append(out, ve.Error())
	}
	return out
}
