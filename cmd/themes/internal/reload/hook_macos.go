package reload

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// macosSidecar is the shape of <theme>/derived/macos.json.
type macosSidecar struct {
	Mode         string `json:"mode"`
	AccentInt    *int   `json:"accent_int"`
	HighlightRGB string `json:"highlight_rgb"`
}

var (
	macosLookPath = exec.LookPath
	macosRun      = func(ctx context.Context, name string, args ...string) error {
		return exec.CommandContext(ctx, name, args...).Run()
	}
)

func runMacOSKillall(ctx context.Context, proc string) {
	killCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	_ = macosRun(killCtx, "killall", proc)
}

// hookMacOS applies the derived macos.json payload to macOS system settings.
//
// Writes `defaults NSGlobalDomain AppleAccentColor / AppleHighlightColor /
// AppleInterfaceStyle` + AppleAquaColorVariant (1 or 6 for Graphite), posts
// Darwin distributed notifications, then restarts cached macOS UI processes:
//
//   - AppleColorPreferencesChangedNotification — accent + highlight
//   - AppleAquaColorVariantChanged            — CoreUI aqua/accent repaint
//   - NSSystemColorsDidChangeNotification     — Tahoe (macOS 26) forward-compat
//
// Dock/SystemUIServer cache accent-backed chrome aggressively. Restart them on
// commit so the Dock/menu bar repaint immediately instead of waiting for the
// next login or manual restart. cfprefsd is restarted after the defaults writes
// so clients re-read fresh preference values.
//
// Mode (dark/light) is set via osascript because `defaults write` alone
// on AppleInterfaceStyle does not fire the appearance-change notification.
// osascript is bounded via the context deadline.
//
// Darwin only. RunPreview=false (accent flash on scroll is jarring),
// RunCommit=true.
func hookMacOS(ctx context.Context, themeDir string) error {
	if runtimeGOOS() != "darwin" {
		return nil // no-op outside macOS
	}

	sidecar := filepath.Join(themeDir, "derived", "macos.json")
	if _, statErr := os.Stat(sidecar); statErr != nil {
		alt := filepath.Join(themeDir, ".macos.json")
		if _, altErr := os.Stat(alt); altErr == nil {
			sidecar = alt
		} else {
			sidecar = ""
		}
	}
	if sidecar == "" {
		return nil
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
		script := `tell app "System Events" to tell appearance preferences to set dark mode to ` + val

		_ = macosRun(modeCtx, "osascript", "-e", script)
		cancel()

		if setDark {
			_ = macosRun(ctx, "defaults", "write", "-g", "AppleInterfaceStyle", "-string", "Dark")
		} else {
			_ = macosRun(ctx, "defaults", "delete", "-g", "AppleInterfaceStyle")
		}
	}

	// --- Accent + Aqua variant --------------------------------------------
	if s.AccentInt != nil {
		accent := strconv.Itoa(*s.AccentInt)
		_ = macosRun(ctx, "defaults", "write", "NSGlobalDomain",
			"AppleAccentColor", "-int", accent)
		// AquaColorVariant: 1 = normal accent, 6 = Graphite override.
		// macOS 26 checks the variant first so both writes are required.
		variant := "1"
		if *s.AccentInt == -1 {
			variant = "6"
		}
		_ = macosRun(ctx, "defaults", "write", "NSGlobalDomain",
			"AppleAquaColorVariant", "-int", variant)
	}

	// --- Highlight color --------------------------------------------------
	if s.HighlightRGB != "" {
		_ = macosRun(ctx, "defaults", "write", "NSGlobalDomain",
			"AppleHighlightColor", "-string", s.HighlightRGB)
	}

	// --- Propagate --------------------------------------------------------
	if _, err := macosLookPath("notifyutil"); err == nil {
		_ = macosRun(ctx, "notifyutil", "-p", "AppleColorPreferencesChangedNotification")
		_ = macosRun(ctx, "notifyutil", "-p", "AppleAquaColorVariantChanged")
		_ = macosRun(ctx, "notifyutil", "-p", "NSSystemColorsDidChangeNotification")
	}

	for _, proc := range []string{"cfprefsd", "Dock", "SystemUIServer"} {
		runMacOSKillall(ctx, proc)
	}

	return nil
}
