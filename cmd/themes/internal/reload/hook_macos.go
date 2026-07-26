package reload

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

// macosSidecar is the shape of <theme>/derived/macos.json.
type macosSidecar struct {
	Mode         string `json:"mode"`
	AccentInt    *int   `json:"accent_int"`
	HighlightRGB string `json:"highlight_rgb"`
}

// hookMacOS ports .hooks/macos-system.sh to Go.
//
// Writes `defaults NSGlobalDomain AppleAccentColor / AppleHighlightColor /
// AppleInterfaceStyle` + AppleAquaColorVariant (1 or 6 for Graphite),
// then posts two Darwin distributed notifications via `notifyutil -p`:
//
//   - AppleColorPreferencesChangedNotification — accent + highlight
//   - NSSystemColorsDidChangeNotification      — Tahoe (macOS 26) forward-compat
//
// The notifyutil posts replace the historical `killall Dock SystemUIServer
// Finder cfprefsd` cascade. That cascade froze the reload orchestrator
// on rapid consecutive theme swaps because macOS serializes SIGKILL
// under WindowServer's window server sync. The distributed notifications
// deliver equivalent repaint to Cocoa apps in <100 ms without any process
// churn.
//
// Mode (dark/light) is set via osascript because `defaults write` alone
// on AppleInterfaceStyle does not fire the appearance-change notification.
// osascript is bounded via the context deadline.
//
// Darwin only. RunPreview=false (accent flash on scroll is jarring),
// RunCommit=true.
func hookMacOS(ctx context.Context, themeDir string) error {
	if runtime.GOOS != "darwin" {
		return nil // no-op on non-Darwin
	}

	sidecar := filepath.Join(themeDir, "derived", "macos.json")
	if _, err := os.Stat(sidecar); err != nil {
		// Fallback to legacy dotfile location for one release.
		alt := filepath.Join(themeDir, ".macos.json")
		if _, err2 := os.Stat(alt); err2 != nil {
			return nil // no payload
		}
		sidecar = alt
	}

	raw, err := os.ReadFile(sidecar)
	if err != nil {
		return fmt.Errorf("hookMacOS: read %s: %w", sidecar, err)
	}
	var s macosSidecar
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("hookMacOS: parse %s: %w", sidecar, err)
	}

	// --- Mode (dark/light) ------------------------------------------------
	// osascript triggers the real appearance-change notification;
	// `defaults write` alone is silent. Bounded via a short-context deadline
	// so a wedged System Events can't stall the hook.
	if s.Mode != "" {
		modeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		setDark := s.Mode == "dark"
		val := "false"
		if setDark {
			val = "true"
		}
		script := fmt.Sprintf(`tell app "System Events" to tell appearance preferences to set dark mode to %s`, val)
		_ = exec.CommandContext(modeCtx, "osascript", "-e", script).Run()
		cancel()

		if setDark {
			_ = exec.CommandContext(ctx, "defaults", "write", "-g", "AppleInterfaceStyle", "-string", "Dark").Run()
		} else {
			_ = exec.CommandContext(ctx, "defaults", "delete", "-g", "AppleInterfaceStyle").Run()
		}
	}

	// --- Accent + Aqua variant --------------------------------------------
	if s.AccentInt != nil {
		accent := strconv.Itoa(*s.AccentInt)
		_ = exec.CommandContext(ctx, "defaults", "write", "NSGlobalDomain",
			"AppleAccentColor", "-int", accent).Run()
		// AquaColorVariant: 1 = normal accent, 6 = Graphite override.
		// macOS 26 checks the variant first so both writes are required.
		variant := "1"
		if *s.AccentInt == -1 {
			variant = "6"
		}
		_ = exec.CommandContext(ctx, "defaults", "write", "NSGlobalDomain",
			"AppleAquaColorVariant", "-int", variant).Run()
	}

	// --- Highlight color --------------------------------------------------
	if s.HighlightRGB != "" {
		_ = exec.CommandContext(ctx, "defaults", "write", "NSGlobalDomain",
			"AppleHighlightColor", "-string", s.HighlightRGB).Run()
	}

	// --- Propagate --------------------------------------------------------
	//
	// Two mechanisms fire together because notification-only propagation
	// is not reliable across every macOS version + Cocoa app matrix.
	// See docs/plans/themes-live-switch-research-2.md § 6.3 for the
	// investigation trail.
	//
	// 1. Bare distributed notifications (Chromium + Alex Chan's recipe):
	//    - AppleColorPreferencesChangedNotification
	//    - AppleAquaColorVariantChanged
	//    Most Cocoa apps that observe accent redraw on these.
	//
	// 2. killall Dock + SystemUIServer:
	//    Restarts the two auto-restarting system daemons that carry the
	//    menu bar clock, Dock icons, and keyboard input indicators. They
	//    hold no user work and auto-relaunch via launchd in ~200ms. This
	//    is what System Preferences itself does when you change accent.
	//
	//    EXPLICITLY NOT KILLED: Finder (modals stall), cfprefsd (cascade
	//    multiplier that trips WindowServer sync on rapid swaps).
	//
	// Users who want notify-only can set THEMES_MACOS_NOTIFY_ONLY=1 to
	// disable the killall path. Rapid consecutive swap regressions cite
	// that env var.
	if _, err := exec.LookPath("notifyutil"); err == nil {
		_ = exec.CommandContext(ctx, "notifyutil", "-p", "AppleColorPreferencesChangedNotification").Run()
		_ = exec.CommandContext(ctx, "notifyutil", "-p", "AppleAquaColorVariantChanged").Run()
	}
	if os.Getenv("THEMES_MACOS_NOTIFY_ONLY") == "" {
		_ = exec.CommandContext(ctx, "killall", "Dock").Run()
		_ = exec.CommandContext(ctx, "killall", "SystemUIServer").Run()
	}
	return nil
}
