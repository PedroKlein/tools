package main

import "testing"

func TestExitCodeValues(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"ExitOK", ExitOK, 0},
		{"ExitError", ExitError, 1},
		{"ExitAmbiguous", ExitAmbiguous, 2},
		{"ExitNotFound", ExitNotFound, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.code, tt.want)
			}
		})
	}
}
