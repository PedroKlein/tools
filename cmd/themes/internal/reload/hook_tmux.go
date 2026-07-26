package reload

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// hookTmux ports .hooks/tmux.sh to Go.
//
//  1. `tmux source-file <theme>/derived/tmux.conf`  — apply theme
//  2. `tmux set -g window-style        bg=default`  — transparency baseline
//  3. `tmux set -g window-active-style bg=default`  — same
//  4. `tmux refresh-client -t <client>` per attached client — force redraw
//
// Steps 2–3 undo the theme emitter's explicit `fg` on window-style,
// which breaks Ghostty/kitty transparency for already-drawn cells.
// Step 4 forces a full pane repaint on every attached client because
// `refresh-client -S` only redraws the status line.
//
// Preview + Commit: RunPreview=true, RunCommit=true. Silently no-ops
// when tmux is not installed or no server is running.
func hookTmux(ctx context.Context, themeDir string) error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return nil // tmux not installed; silent no-op
	}

	src := filepath.Join(themeDir, "derived", "tmux.conf")
	if _, err := os.Stat(src); err != nil {
		alt := filepath.Join(themeDir, "tmux.conf")
		if _, err2 := os.Stat(alt); err2 != nil {
			return nil // no tmux.conf; silent no-op
		}
		src = alt
	}

	// Fire source-file first. If tmux isn't running, this exits
	// non-zero — we swallow the error (mirroring shell `|| true`).
	_ = exec.CommandContext(ctx, "tmux", "source-file", src).Run()

	// Transparency baseline. Same swallow-error semantics.
	_ = exec.CommandContext(ctx, "tmux", "set", "-g", "window-style", "bg=default").Run()
	_ = exec.CommandContext(ctx, "tmux", "set", "-g", "window-active-style", "bg=default").Run()

	// Enumerate attached clients and refresh each. `list-clients` output
	// looks like:  /dev/ttys002  or one client per line by client name.
	// We use -F '#{client_name}' for stability.
	out, err := exec.CommandContext(ctx, "tmux", "list-clients", "-F", "#{client_name}").Output()
	if err != nil {
		return nil // no server, nothing to refresh
	}
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		_ = exec.CommandContext(ctx, "tmux", "refresh-client", "-t", name).Run()
	}
	return nil
}
