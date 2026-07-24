package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// jsonOutput is set by parseGlobalFlags when --json is present.
var jsonOutput bool

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
	_ = enc.Encode(out)
	os.Exit(code)
}

// dieMsg either writes a JSON error (--json) or prints to stderr and exits.
func dieMsg(msg string, code int) {
	if jsonOutput {
		writeJSONError(msg, code)
	}
	fmt.Fprintln(os.Stderr, "themes:", msg)
	os.Exit(code)
}
