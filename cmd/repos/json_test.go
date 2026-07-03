package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	type sample struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	tests := []struct {
		name string
		val  any
		want string
	}{
		{
			name: "struct",
			val:  sample{Name: "test", Count: 42},
			want: `{
  "name": "test",
  "count": 42
}
`,
		},
		{
			name: "array",
			val:  []sample{{Name: "a", Count: 1}, {Name: "b", Count: 2}},
			want: `[
  {
    "name": "a",
    "count": 1
  },
  {
    "name": "b",
    "count": 2
  }
]
`,
		},
		{
			name: "empty array",
			val:  []sample{},
			want: "[]\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := captureStdout(func() {
				writeJSON(tt.val)
			})
			if got != tt.want {
				t.Errorf("writeJSON() output:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestWriteJSON_ValidJSON(t *testing.T) {
	type entry struct {
		Path string `json:"path"`
	}

	got := captureStdout(func() {
		writeJSON([]entry{{Path: "github.com/owner/repo"}})
	})

	if !json.Valid([]byte(got)) {
		t.Errorf("writeJSON() produced invalid JSON: %s", got)
	}
}

// captureStdout captures stdout output from a function.
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close() //nolint:gosec // test helper pipe, close error is non-actionable

	os.Stdout = old

	var buf bytes.Buffer

	_, _ = io.Copy(&buf, r)

	return buf.String()
}
