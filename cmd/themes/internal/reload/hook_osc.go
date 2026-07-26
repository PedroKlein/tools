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

type liveApplyContextKey struct{}

var controllingTTYPath = "/dev/tty"

// terminalPrograms is the list of GUI terminal parent binaries whose
// children own a real TTY we can broadcast to.
var terminalPrograms = []string{"ghostty", "kitty", "alacritty", "wezterm", "foot", "iTerm2"}

// hookOSC ports .hooks/osc-broadcast.sh to Go.
//
// Commit retints open terminal panes by emitting OSC escape sequences to
// discovered TTYs. Preview writes only to the picker's controlling terminal
// (/dev/tty); enumerating every Ghostty child during cursor movement made
// one terminal's picker retint sibling windows.
//
// Broadcast retints send OSC 11 to direct terminal TTYs and tmux client
// TTYs, but not tmux pane TTYs. The client TTY updates Ghostty's outer
// window background; pane TTYs get fg/cursor/ANSI frames only because
// OSC 11 there makes default-bg tmux cells lose window opacity.
//
// Emits:
//   - OSC 10;<fg>     — default foreground
//   - OSC 11;<bg>     — default background, preview-only outside tmux
//   - OSC 12;<cursor> — cursor color
//   - OSC 4;N;<hex>   — ANSI slot N (0..15)
//   - OSC 1337 SetColors=fg=<hex>,bg=<hex>,cursor=<hex>,ansi0=<hex>,...
//     (iTerm2 proprietary — non-supporting terminals ignore silently)
//
// Follow-up: SIGWINCH the foreground process group of commit-discovered TTYs
// so TUI apps (nvim, less, btop, k9s) receive a redraw signal.
//
// Preview + Commit: RunPreview=true, RunCommit=true. Target <50ms on a
// 6-pane setup.
func hookOSC(ctx context.Context, themeDir string) error {
	// 1. Load theme.json and build the blob.
	t, err := palette.Load(themeDir)
	if err != nil {
		return fmt.Errorf("hookOSC: load %s: %w", themeDir, err)
	}
	liveApply := isLiveApply(ctx)
	fullBlob := buildOSCBlobWithOptions(t, oscOptions{IncludeBackground: true})
	if len(fullBlob) == 0 {
		return nil
	}
	noBackgroundBlob := buildOSCBlobWithOptions(t, oscOptions{IncludeBackground: false})

	if liveApply {
		if os.Getenv("TMUX") == "" {
			return writeOSCToControllingTTY(fullBlob)
		}
		if clientTTY := currentTmuxClientTTY(ctx); clientTTY != "" {
			_ = writeOSCToTTY(clientTTY, fullBlob)
		}
		return writeOSCToControllingTTY(noBackgroundBlob)
	}

	tmuxPanes, tmuxClients := discoverTmuxTTYs(ctx)
	directTTYs := discoverTerminalTTYs(ctx, tmuxClients)
	fullTTYs, noBackgroundTTYs := commitOSCTargets(directTTYs, tmuxClients, tmuxPanes)
	broadcastToTTYs(ctx, fullTTYs, fullBlob)
	broadcastToTTYs(ctx, noBackgroundTTYs, noBackgroundBlob)

	// 4. Refresh tmux (retints status bar).
	_ = tmuxCommand(ctx, "refresh-client", "-S").Run()
	return nil
}

// oscOptions controls which terminal-level colors are safe for a context.
type oscOptions struct {
	IncludeBackground bool
}

// buildOSCBlob assembles the default full OSC sequence for tests and commit
// paths that are not running inside tmux.
func buildOSCBlob(t *palette.Theme) []byte {
	return buildOSCBlobWithOptions(t, oscOptions{IncludeBackground: true})
}

// buildOSCBlobWithOptions assembles the concatenated OSC escape sequence for
// the given theme. Result is a single byte slice callers write() unchanged.
// Empty when theme.json is missing required fields.
func buildOSCBlobWithOptions(t *palette.Theme, opts oscOptions) []byte {
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
	if opts.IncludeBackground {
		writeOSC("11", t.Palette.Semantic.Bg)
	}
	cursor := t.Palette.Semantic.Cursor
	if cursor == "" {
		cursor = t.Palette.Semantic.Fg
	}
	writeOSC("12", cursor)
	// ANSI 0..15.
	for i := range 16 {
		if t.Palette.Ansi[i] != "" {
			writeOSC("4;"+strconv.Itoa(i), t.Palette.Ansi[i])
		}
	}
	// OSC 1337 SetColors= (iTerm2). One frame with all safe keys.
	setColors := buildSetColors(t, cursor, opts.IncludeBackground)
	if setColors != "" {
		fmt.Fprintf(&buf, "%s]1337;SetColors=%s%s", esc, setColors, bel)
	}
	return buf.Bytes()
}

// buildSetColors returns the comma-separated key=value payload for the
// iTerm2 OSC 1337 SetColors= frame. Hex values MUST NOT have a leading
// '#' per the iTerm2 spec.
func buildSetColors(t *palette.Theme, cursor string, includeBackground bool) string {
	strip := func(s string) string { return strings.TrimPrefix(s, "#") }
	pairs := []string{
		"fg=" + strip(t.Palette.Semantic.Fg),
	}
	if includeBackground {
		pairs = append(pairs, "bg="+strip(t.Palette.Semantic.Bg))
	}
	pairs = append(pairs, "cursor="+strip(cursor))
	for i := range 16 {
		if t.Palette.Ansi[i] == "" {
			continue
		}
		pairs = append(pairs, fmt.Sprintf("ansi%d=%s", i, strip(t.Palette.Ansi[i])))
	}
	return strings.Join(pairs, ",")
}

func isLiveApply(ctx context.Context) bool {
	live, _ := ctx.Value(liveApplyContextKey{}).(bool)
	return live
}

func writeOSCToControllingTTY(blob []byte) error {
	return writeOSCToTTY(controllingTTYPath, blob)
}

func writeOSCToTTY(tty string, blob []byte) error {
	f, err := os.OpenFile(tty, os.O_WRONLY, 0)
	if err != nil {
		return nil
	}
	_, _ = io.Copy(f, bytes.NewReader(blob))
	return f.Close()
}

var currentTmuxClientTTY = func(ctx context.Context) string {
	out, err := tmuxCommand(ctx, "display-message", "-p", "#{client_tty}").Output()
	if err != nil {
		return ""
	}
	tty := strings.TrimSpace(string(out))
	if !strings.HasPrefix(tty, "/dev/") {
		return ""
	}
	return tty
}

// discoverTTYs returns all OSC targets and is kept for legacy tests/helpers.
// New hook code uses discoverTmuxTTYs + discoverTerminalTTYs so background
// OSC frames can be scoped away from tmux pane TTYs.
func discoverTTYs(ctx context.Context) []string {
	tmuxPanes, tmuxClients := discoverTmuxTTYs(ctx)
	direct := discoverTerminalTTYs(ctx, tmuxClients)
	seen := map[string]struct{}{}
	for _, tty := range tmuxPanes {
		addTTY(seen, tty)
	}
	for _, tty := range direct {
		addTTY(seen, tty)
	}
	return sortedTTYs(seen)
}

func discoverTmuxTTYs(ctx context.Context) (panes []string, clients map[string]struct{}) {
	panesSeen := map[string]struct{}{}
	clientSeen := map[string]struct{}{}
	if _, err := exec.LookPath("tmux"); err != nil {
		return nil, clientSeen
	}
	if out, err := exec.CommandContext(ctx, "tmux", "list-panes", "-a", "-F", "#{pane_tty}").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			addTTY(panesSeen, line)
		}
	}
	if out, err := exec.CommandContext(ctx, "tmux", "list-clients", "-F", "#{client_tty}").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			addTTY(clientSeen, line)
		}
	}
	return sortedTTYs(panesSeen), clientSeen
}

func discoverTerminalTTYs(ctx context.Context, skip map[string]struct{}) []string {
	seen := map[string]struct{}{}
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
				addTTY(seen, "/dev/tty"+name)
			}
		}
	}
	return filterTTYs(sortedTTYs(seen), skip)
}

func addTTY(seen map[string]struct{}, tty string) {
	tty = strings.TrimSpace(tty)
	if tty == "" || !strings.HasPrefix(tty, "/dev/") {
		return
	}
	seen[tty] = struct{}{}
}

func filterTTYs(ttys []string, skip map[string]struct{}) []string {
	if len(skip) == 0 {
		return ttys
	}
	out := ttys[:0]
	for _, tty := range ttys {
		if _, ok := skip[tty]; ok {
			continue
		}
		out = append(out, tty)
	}
	return out
}

func sortedTTYs(seen map[string]struct{}) []string {
	out := make([]string, 0, len(seen))
	for tty := range seen {
		out = append(out, tty)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

func commitOSCTargets(directTTYs []string, tmuxClients map[string]struct{}, tmuxPanes []string) (full []string, noBackground []string) {
	full = append(full, directTTYs...)
	full = append(full, sortedTTYs(tmuxClients)...)
	noBackground = append(noBackground, tmuxPanes...)
	return full, noBackground
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
