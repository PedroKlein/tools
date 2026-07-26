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
//  4. `tmux refresh-client -S -t <client>` per attached client — redraw status
//
// Steps 2–3 undo the theme emitter's explicit `fg` on window-style,
// which breaks Ghostty/kitty transparency for already-drawn cells.
// Step 4 intentionally uses status-only refresh; a full client repaint can
// make existing Ghostty-backed tmux panes render a solid default background.
//
// Preview + Commit: RunPreview=true, RunCommit=true. Preview only runs when
// invoked from inside tmux, and it is scoped to the current $TMUX socket.
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

	if isLiveApply(ctx) && tmuxSocketFromEnv() == "" {
		return nil
	}

	// Fire source-file first. If tmux isn't running, this exits
	// non-zero — we swallow the error (mirroring shell `|| true`).
	_ = tmuxCommand(ctx, "source-file", src).Run()

	// Transparency baseline. Same swallow-error semantics.
	_ = tmuxCommand(ctx, "set", "-g", "window-style", "bg=default").Run()
	_ = tmuxCommand(ctx, "set", "-g", "window-active-style", "bg=default").Run()

	out, err := tmuxCommand(ctx, "list-clients", "-F", "#{client_name}").Output()
	if err != nil {
		return nil // no server, nothing to refresh
	}
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		_ = tmuxCommand(ctx, "refresh-client", "-S", "-t", name).Run()
	}
	return nil
}

func tmuxCommand(ctx context.Context, args ...string) *exec.Cmd {
	if socket := tmuxSocketFromEnv(); socket != "" {
		args = append([]string{"-S", socket}, args...)
	}
	return exec.CommandContext(ctx, "tmux", args...)
}

func tmuxSocketFromEnv() string {
	value := os.Getenv("TMUX")
	if value == "" {
		return ""
	}
	socket, _, _ := strings.Cut(value, ",")
	if !strings.HasPrefix(socket, "/") {
		return ""
	}
	return socket
}
