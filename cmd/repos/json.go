package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// writeJSON marshals v as indented JSON to stdout.
func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
		os.Exit(ExitError)
	}
}

// writeJSONError writes a JSON error object to stdout and exits with the given code.
func writeJSONError(msg string, code int) {
	out := struct {
		Error string `json:"error"`
	}{Error: msg}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding error JSON: %v\n", err)
	}

	os.Exit(code)
}
