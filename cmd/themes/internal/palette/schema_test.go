package palette

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// schemaPath resolves ../../schema/theme.schema.json relative to this file.
// Test cwd is cmd/themes/internal/palette/, so ../../schema/... reaches
// cmd/themes/schema/theme.schema.json.
func schemaPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "schema", "theme.schema.json")
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("resolve schema path: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("schema not found at %s: %v", abs, err)
	}
	return abs
}

func loadSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020
	s, err := compiler.Compile(schemaPath(t))
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return s
}

func loadTheme(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return m
}

// TestSchemaValidatesReferenceThemes is the positive-case gate: the
// hand-authored osaka-jade reference must validate against the frozen
// v1 schema.
func TestSchemaValidatesReferenceThemes(t *testing.T) {
	schema := loadSchema(t)

	themes := []string{
		filepath.Join("testdata", "osaka-jade", "theme.json"),
	}
	for _, path := range themes {
		theme := loadTheme(t, path)
		if err := schema.Validate(theme); err != nil {
			t.Errorf("%s should validate but did not:\n%v", path, err)
		}
	}
}

// TestSchemaRejectsMissingRequiredSemanticSlots verifies that removing
// any one of the 7 required semantic slots fails validation, and fails
// for the right reason (that slot's `required` clause).
func TestSchemaRejectsMissingRequiredSemanticSlots(t *testing.T) {
	schema := loadSchema(t)
	base := loadTheme(t, filepath.Join("testdata", "osaka-jade", "theme.json"))

	requiredSlots := []string{"bg", "fg", "muted", "accent", "error", "warning", "ok"}
	for _, slot := range requiredSlots {
		t.Run(slot, func(t *testing.T) {
			// Deep-clone by JSON round-trip to avoid mutating the shared map.
			raw, err := json.Marshal(base)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var mutant map[string]any
			if err := json.Unmarshal(raw, &mutant); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			semantic := mutant["palette"].(map[string]any)["semantic"].(map[string]any)
			delete(semantic, slot)

			err = schema.Validate(mutant)
			if err == nil {
				t.Fatalf("mutant missing semantic.%s should have failed validation", slot)
			}
			// jsonschema/v5 embeds the offending key name in the error tree.
			// Assert the missing-slot name appears in the message so we know
			// the failure is for the right reason.
			if !strings.Contains(err.Error(), slot) {
				t.Errorf("mutant missing semantic.%s failed but not for that reason:\n%v", slot, err)
			}
		})
	}
}

// TestSchemaPreservesUnknownTopLevelKeys documents the "schema evolution"
// invariant: adding an unknown top-level key does not fail validation.
// The loader package relies on this to preserve forward-compat fields.
func TestSchemaPreservesUnknownTopLevelKeys(t *testing.T) {
	schema := loadSchema(t)
	base := loadTheme(t, filepath.Join("testdata", "osaka-jade", "theme.json"))
	base["experimental_key"] = "future-proof-me"
	if err := schema.Validate(base); err != nil {
		t.Fatalf("unknown top-level key should be tolerated: %v", err)
	}
}

// TestSchemaRejectsBadAnsiLength documents the ANSI array constraint:
// the emitters address entries by index [0..15], so anything shorter
// must fail loud rather than silently truncate.
func TestSchemaRejectsBadAnsiLength(t *testing.T) {
	schema := loadSchema(t)
	base := loadTheme(t, filepath.Join("testdata", "osaka-jade", "theme.json"))
	palette := base["palette"].(map[string]any)
	ansi := palette["ansi"].([]any)
	palette["ansi"] = ansi[:15]
	if err := schema.Validate(base); err == nil {
		t.Fatal("15-entry ansi should have failed")
	}
}
