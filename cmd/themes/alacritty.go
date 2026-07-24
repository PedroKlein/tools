package main

import (
	"bufio"
	"os"
	"strings"
)

// parseAlacrittyForSwatch returns a flat name->hex map with keys:
//
//	primary_bg, primary_fg, cursor_cursor, cursor_text,
//	normal_black, ..., bright_white
//
// It is a minimal, permissive TOML reader tuned for alacritty.toml's
// [colors.*] sections. Returns partial results on malformed input rather
// than failing; the TUI can still render whatever palette parts it found.
func parseAlacrittyForSwatch(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := map[string]string{}
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			// e.g. [colors.normal] -> section = normal
			s := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			s = strings.TrimPrefix(s, "colors.")
			section = s
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `"'`)
		if !strings.HasPrefix(val, "#") {
			continue
		}
		out[section+"_"+key] = strings.ToUpper(val)
	}
	return out, sc.Err()
}
