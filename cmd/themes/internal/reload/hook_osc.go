package reload

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/PedroKlein/tools/cmd/themes/internal/palette"
)

const (
	esc = "\x1b"
	bel = "\x07"
)

// terminalPrograms is the list of GUI terminal parent binaries whose
// children own a real TTY we can broadcast to.
var terminalPrograms = []string{"ghostty", "kitty", "alacritty", "wezterm", "foot", "iTerm2"}

// hookOSC ports .hooks/osc-broadcast.sh to Go.
//
// Live-retint every open terminal pane by emitting OSC escape sequences
// to each pane's TTY. Every VT100-compliant terminal (Ghostty, kitty,
// alacritty, foot, iTerm2, wezterm) interprets these and updates the
// palette instantly. ANSI-colored output (ls, git, zsh-syntax-highlight)
// re-renders on next redraw with no shell reload.
//
// Emits:
//   - OSC 10;<fg>     — default foreground
//   - OSC 11;<bg>     — default background
//   - OSC 12;<cursor> — cursor color
//   - OSC 4;N;<hex>   — ANSI slot N (0..15)
//   - OSC 1337 SetColors=fg=<hex>,bg=<hex>,cursor=<hex>,ansi0=<hex>,...
//     (iTerm2 proprietary — non-supporting terminals ignore silently)
//
// TTY discovery: tmux `list-panes -a` + `pgrep <term>` walk to child PIDs
// then `ps -o tt=` for each. Combined + deduplicated.
//
// Broadcast: concurrent goroutines + os.WriteFile (plan constraint:
// MUST NOT fork a subprocess per pane).
//
// Follow-up: SIGWINCH the foreground process group of each TTY so TUI
// apps (nvim, less, btop, k9s) receive a redraw signal.
//
// Preview + Commit: RunPreview=true, RunCommit=true. Target <50ms on a
// 6-pane setup.
func hookOSC(ctx context.Context, themeDir string) error {
	// 1. Load theme.json and build the blob.
	t, err := palette.Load(themeDir)
	if err != nil {
		return fmt.Errorf("hookOSC: load %s: %w", themeDir, err)
	}
	blob := buildOSCBlob(t)
	if len(blob) == 0 {
		return nil
	}

	// 2. Discover TTYs.
	ttys := discoverTTYs(ctx)
	if len(ttys) == 0 {
		return nil
	}

	// 3. Concurrent broadcast + SIGWINCH.
	broadcastToTTYs(ctx, ttys, blob)

	// 4. Refresh tmux (retints status bar).
	_ = exec.CommandContext(ctx, "tmux", "refresh-client", "-S").Run()
	return nil
}

// buildOSCBlob assembles the concatenated OSC escape sequence for the
// given theme. Result is a single byte slice callers write() unchanged
// to each TTY. Empty when theme.json is missing required fields.
func buildOSCBlob(t *palette.Theme) []byte {
	if t == nil {
		return nil
	}
	var buf bytes.Buffer
	writeOSC := func(code, val string) {
		if val == "" {
			return
		}
		fmt.Fprintf(&buf, "%s]%s;%s%s", esc, code, val, bel)
	}
	// Default fg/bg/cursor.
	writeOSC("10", t.Palette.Semantic.Fg)
	writeOSC("11", t.Palette.Semantic.Bg)
	cursor := t.Palette.Semantic.Cursor
	if cursor == "" {
		cursor = t.Palette.Semantic.Fg
	}
	writeOSC("12", cursor)
	// ANSI 0..15.
	for i := 0; i < 16; i++ {
		if t.Palette.Ansi[i] != "" {
			writeOSC("4;"+strconv.Itoa(i), t.Palette.Ansi[i])
		}
	}
	// OSC 1337 SetColors= (iTerm2). One frame with all keys.
	setColors := buildSetColors(t, cursor)
	if setColors != "" {
		fmt.Fprintf(&buf, "%s]1337;SetColors=%s%s", esc, setColors, bel)
	}
	return buf.Bytes()
}

// buildSetColors returns the comma-separated key=value payload for the
// iTerm2 OSC 1337 SetColors= frame. Hex values MUST NOT have a leading
// '#' per the iTerm2 spec.
func buildSetColors(t *palette.Theme, cursor string) string {
	strip := func(s string) string { return strings.TrimPrefix(s, "#") }
	pairs := []string{
		"fg=" + strip(t.Palette.Semantic.Fg),
		"bg=" + strip(t.Palette.Semantic.Bg),
		"cursor=" + strip(cursor),
	}
	for i := 0; i < 16; i++ {
		if t.Palette.Ansi[i] == "" {
			continue
		}
		pairs = append(pairs, fmt.Sprintf("ansi%d=%s", i, strip(t.Palette.Ansi[i])))
	}
	return strings.Join(pairs, ",")
}

// discoverTTYs returns the union of tmux pane TTYs and terminal-child
// TTYs, deduplicated. Both discovery paths tolerate missing binaries.
func discoverTTYs(ctx context.Context) []string {
	seen := map[string]struct{}{}
	add := func(tty string) {
		tty = strings.TrimSpace(tty)
		if tty == "" || !strings.HasPrefix(tty, "/dev/") {
			return
		}
		seen[tty] = struct{}{}
	}

	// tmux panes.
	if _, err := exec.LookPath("tmux"); err == nil {
		out, _ := exec.CommandContext(ctx, "tmux", "list-panes", "-a", "-F", "#{pane_tty}").Output()
		for _, line := range strings.Split(string(out), "\n") {
			add(line)
		}
	}

	// Terminal children: pgrep <prog>, then children of those, then ps -o tt= per child.
	for _, prog := range terminalPrograms {
		parents, err := exec.CommandContext(ctx, "pgrep", "-x", prog).Output()
		if err != nil || len(bytes.TrimSpace(parents)) == 0 {
			continue
		}
		for _, ppid := range strings.Fields(string(parents)) {
			kids, _ := exec.CommandContext(ctx, "pgrep", "-P", ppid).Output()
			for _, k := range strings.Fields(string(kids)) {
				tt, _ := exec.CommandContext(ctx, "ps", "-o", "tt=", "-p", k).Output()
				name := strings.TrimSpace(string(tt))
				if name == "" || name == "??" {
					continue
				}
				add("/dev/tty" + name)
			}
		}
	}

	// Sort for determinism (helps testing).
	out := make([]string, 0, len(seen))
	for tty := range seen {
		out = append(out, tty)
	}
	// Simple insertion sort — set is tiny in practice.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// broadcastToTTYs writes the blob to every TTY concurrently. Failures
// (pane closed since scan, permission denied) are swallowed. After a
// successful write, SIGWINCH the TTY's foreground process group so TUI
// apps redraw.
func broadcastToTTYs(ctx context.Context, ttys []string, blob []byte) {
	var wg sync.WaitGroup
	for _, tty := range ttys {
		wg.Add(1)
		go func(tty string) {
			defer wg.Done()
			f, err := os.OpenFile(tty, os.O_WRONLY, 0)
			if err != nil {
				return
			}
			// Best-effort write; ignore errors.
			_, _ = io.Copy(f, bytes.NewReader(blob))
			_ = f.Close()

			// SIGWINCH to the pane's foreground process group.
			sigwinchTTY(ctx, tty)
		}(tty)
	}
	wg.Wait()
}

// sigwinchTTY sends SIGWINCH to the foreground process group of the
// given TTY. Uses `ps -o tpgid= -t <tty-name>` to discover the tpgid,
// then syscall.Kill(-tpgid, SIGWINCH).
//
// Failures (no fg pgid, permission denied) are swallowed.
func sigwinchTTY(ctx context.Context, tty string) {
	name := strings.TrimPrefix(tty, "/dev/")
	out, err := exec.CommandContext(ctx, "ps", "-o", "tpgid=", "-t", name).Output()
	if err != nil {
		return
	}
	// ps returns one tpgid per line; take the first non-zero value.
	var tpgid int
	for _, line := range strings.Split(string(out), "\n") {
		v, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && v > 0 {
			tpgid = v
			break
		}
	}
	if tpgid <= 0 {
		return
	}
	_ = syscall.Kill(-tpgid, syscall.SIGWINCH)
}

// _themeRoot silences the runtime import when tests build against no
// palette themes (older Go versions warn otherwise). Reserved for
// future no-op guards.
var _ = filepath.Base
var _ = runtime.GOOS
