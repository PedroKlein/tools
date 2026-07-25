package reload

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// registry is the canonical hook list. Kept alphabetical for readability.
// New hooks: append and update tests. Anything in the .hooks/ directory
// with an .sh extension that isn't listed here is discovered dynamically
// (see LoadExternalHooks) so the registry doesn't have to enumerate every
// old script.
//
// LiveApply semantics: the picker's scroll preview fires with
// THEME_LIVE_APPLY=1. Hooks with LiveApply=false are skipped there so
// scroll stays snappy. Rule of thumb: if the hook only takes effect on
// app relaunch, LiveApply=false (no point retinting an app that isn't
// visibly re-tinted yet).
var registry = []Hook{
	// --- Inline (Go) hooks -----------------------------------------------

	{
		Name:      "bat",
		Kind:      KindCommand,
		Cmd:       "bat",
		Args:      []string{"cache", "--build"},
		LiveApply: false, // bat re-reads on next invocation; no point in scroll
	},
	{
		Name:      "btop",
		Kind:      KindNoop,
		LiveApply: false, // btop only reads config at launch
	},
	{
		Name:      "gh-dash",
		Kind:      KindNoop,
		LiveApply: false,
	},
	{
		Name:      "ghostty",
		Kind:      KindInline,
		Fn:        reloadGhostty,
		LiveApply: true, // OSC broadcast handles running panes; this covers future panes
	},
	{
		Name:      "k9s",
		Kind:      KindNoop,
		LiveApply: false,
	},
	{
		Name:      "lazygit",
		Kind:      KindNoop,
		LiveApply: false,
	},
	{
		Name:         "nvim",
		Kind:         KindSignal,
		Signal:       "SIGUSR1",
		SignalTarget: "nvim",
		LiveApply:    true, // running nvim instances retint via Signal SIGUSR1 autocmd + theme_loader.reload()
	},
	{
		Name:      "opencode",
		Kind:      KindInline,
		Fn:        reloadOpencode,
		LiveApply: false, // opencode reads tui.json on next launch
	},
	{
		Name:      "sketchybar",
		Kind:      KindCommand,
		Cmd:       "sketchybar",
		Args:      []string{"--reload"},
		LiveApply: true, // visible immediately in the top bar
	},
	{
		Name:      "television",
		Kind:      KindNoop,
		LiveApply: false,
	},

	// --- External (.sh) hooks --------------------------------------------
	// These stay as shell scripts because they benefit from shell idioms
	// (osc-broadcast: awk + perl + kill loops; wallpaper: multi-tier
	// fallback chain; pi: jq multi-file byte-copy; macos-system: defaults
	// write chain; tmux: tmux env var propagation).

	{
		Name:      "osc-broadcast",
		Kind:      KindExternal,
		Script:    "osc-broadcast.sh",
		LiveApply: true, // core of live retint; must fire on scroll
	},
	{
		Name:      "macos-system",
		Kind:      KindExternal,
		Script:    "macos-system.sh",
		LiveApply: true, // .macos.json read is fast; safe on scroll
	},
	{
		Name:      "pi",
		Kind:      KindExternal,
		Script:    "pi.sh",
		LiveApply: true, // pi's file watcher hot-reloads on byte overwrite
	},
	{
		Name:      "tmux",
		Kind:      KindExternal,
		Script:    "tmux.sh",
		LiveApply: true, // status bar retints instantly
	},
	{
		Name:      "wallpaper",
		Kind:      KindExternal,
		Script:    "wallpaper.sh",
		LiveApply: true, // desktop is visible on scroll
	},
}

// --- Inline hook implementations ------------------------------------------

// reloadGhostty picks up new colors in future Ghostty panes (running panes
// use the OSC broadcast; this just makes the config valid for new launches).
//
// Ghostty on macOS re-reads config on SIGUSR2. On Linux with systemd
// (Omarchy), the service unit is reloaded to trigger the same effect.
func reloadGhostty(ctx context.Context, themeDir string) error {
	// systemd path first (Omarchy).
	if _, err := exec.LookPath("systemctl"); err == nil {
		out, err := exec.Command("systemctl", "--user", "is-active", "com.mitchellh.ghostty.service").Output()
		if err == nil && strings.TrimSpace(string(out)) == "active" {
			_ = exec.Command("systemctl", "reload", "--user", "com.mitchellh.ghostty.service").Run()
			return nil
		}
	}
	// Signal every running ghostty (no-op if none).
	return signalProcess("ghostty", syscall.SIGUSR2)
}

// reloadOpencode rewrites ~/.config/opencode/tui.json's `theme` field to
// match the current theme's opencode.name file.
//
// Idempotent: no-op if the current theme already matches. Uses a temp
// file + rename so a crash mid-write doesn't corrupt tui.json.
func reloadOpencode(ctx context.Context, themeDir string) error {
	nameFile := filepath.Join(themeDir, "opencode.name")
	nameData, err := os.ReadFile(nameFile)
	if err != nil {
		return nil // no opencode.name for this theme
	}
	newTheme := strings.TrimSpace(string(nameData))
	if newTheme == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	tuiPath := filepath.Join(home, ".config", "opencode", "tui.json")
	data, err := os.ReadFile(tuiPath)
	if err != nil {
		return nil // no opencode config
	}
	var tui map[string]any
	if err := json.Unmarshal(data, &tui); err != nil {
		return fmt.Errorf("opencode tui.json parse: %w", err)
	}
	if current, ok := tui["theme"].(string); ok && current == newTheme {
		return nil // already applied
	}
	tui["theme"] = newTheme
	out, err := json.MarshalIndent(tui, "", "  ")
	if err != nil {
		return err
	}
	// Atomic write: temp + rename.
	tmp, err := os.CreateTemp(filepath.Dir(tuiPath), "tui.json.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(append(out, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	tmp.Close()
	return os.Rename(tmpPath, tuiPath)
}

// signalProcess sends sig to every process whose /proc name (or macOS
// equivalent via ps) matches target. Silent no-op when no match.
//
// Uses `pkill` if available (works on Linux and macOS); falls back to
// scanning /proc on Linux for headless / minimal-tool environments.
func signalProcess(target string, sig syscall.Signal) error {
	// pkill path — universally available on our target platforms.
	if _, err := exec.LookPath("pkill"); err == nil {
		sigName := signalName(sig)
		// pkill exits 1 when no processes match; that's fine.
		_ = exec.Command("pkill", "-"+sigName, target).Run()
		return nil
	}
	// Last resort: scan /proc (Linux only). No-op on Darwin without pkill,
	// which is unrealistic (pkill ships with macOS since 10.8).
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		pid := e.Name()
		if pid == "" || pid[0] < '0' || pid[0] > '9' {
			continue
		}
		comm, err := os.ReadFile("/proc/" + pid + "/comm")
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(comm)) == target {
			var p int
			fmt.Sscanf(pid, "%d", &p)
			if proc, err := os.FindProcess(p); err == nil {
				_ = proc.Signal(sig)
			}
		}
	}
	return nil
}

// signalName maps syscall.Signal to its short name for pkill.
func signalName(sig syscall.Signal) string {
	switch sig {
	case syscall.SIGUSR1:
		return "SIGUSR1"
	case syscall.SIGUSR2:
		return "SIGUSR2"
	case syscall.SIGHUP:
		return "SIGHUP"
	case syscall.SIGTERM:
		return "SIGTERM"
	}
	return fmt.Sprintf("%d", int(sig))
}

// defaultTimeout is used when Hook.Timeout is zero.
const defaultTimeout = 4 * time.Second
