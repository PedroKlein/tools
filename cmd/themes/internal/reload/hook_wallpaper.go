package reload

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	xdgstate "github.com/PedroKlein/tools/cmd/themes/internal/state"
)

// hookWallpaper applies the wallpaper recorded in XDG state for themeDir.
//
// Resolves the wallpaper via (in order):
//  1. state.json .wallpaper (via internal/state.Load)
//  2. First image under <theme>/backgrounds/ (jpg/jpeg/png/webp)
//  3. No-op
//
// Setter chain, macOS:
//   - desktoppr (preferred; no Automation permission prompt, all Spaces)
//   - osascript System Events (fallback ~500ms)
//   - osascript Finder (last resort ~1-2s)
//
// Linux (best effort):
//   - swww img
//   - hyprctl hyprpaper reload
//
// Commit resolves state.Wallpaper first. PreviewWallpaper below uses preview
// resolution so a selected theme can show its wallpaper without writing
// state.json.
//
// PreviewWallpaper applies themeDir's wallpaper using preview resolution.
// It is called by the picker after a debounce instead of participating in
// the synchronous RunPreview hook wave.
func PreviewWallpaper(ctx context.Context, themeDir string) error {
	return hookWallpaper(context.WithValue(ctx, liveApplyContextKey{}, true), themeDir)
}

// hookWallpaper applies the wallpaper selected by commit or preview mode.
func hookWallpaper(ctx context.Context, themeDir string) error {
	wp := resolveWallpaperForMode(themeDir, isLiveApply(ctx))
	if wp == "" {
		return nil
	}
	if _, err := os.Stat(wp); err != nil {
		return fmt.Errorf("hookWallpaper: no such file: %s", wp)
	}

	switch runtime.GOOS {
	case "darwin":
		return setWallpaperDarwin(ctx, wp)
	case "linux":
		return setWallpaperLinux(ctx, wp)
	default:
		return nil // unsupported OS
	}
}

// resolveWallpaper implements the commit lookup: XDG state.json, then
// first-image-in-backgrounds.
func resolveWallpaper(themeDir string) string {
	return resolveWallpaperForMode(themeDir, false)
}

func resolveWallpaperForMode(themeDir string, liveApply bool) string {
	if liveApply {
		return resolveWallpaperPreview(themeDir)
	}
	// 1. XDG state.
	if s, err := xdgstate.Load(); err == nil && s != nil && s.Wallpaper != "" {
		return s.Wallpaper
	}
	// 2. First background in the theme dir.
	return firstWallpaperInTheme(themeDir)
}

func resolveWallpaperPreview(themeDir string) string {
	if s, err := xdgstate.Load(); err == nil && s != nil {
		if wp := s.WallpaperByTheme[themeName(themeDir)]; wp != "" {
			return wp
		}
	}
	return firstWallpaperInTheme(themeDir)
}

func firstWallpaperInTheme(themeDir string) string {
	bg := filepath.Join(themeDir, "backgrounds")
	entries, err := os.ReadDir(bg)
	if err != nil {
		return ""
	}
	// Sort by name for determinism.
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") ||
			strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".webp") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return ""
	}
	// Simple insertion sort.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j-1] > names[j]; j-- {
			names[j-1], names[j] = names[j], names[j-1]
		}
	}
	return filepath.Join(bg, names[0])
}

func setWallpaperDarwin(ctx context.Context, wp string) error {
	// Preferred: desktoppr. Handles all Spaces, no Automation prompt.
	if _, err := exec.LookPath("desktoppr"); err == nil {
		// Idempotence: skip when already set.
		out, _ := exec.CommandContext(ctx, "desktoppr").Output()
		if strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]) == wp {
			return nil
		}
		if err := exec.CommandContext(ctx, "desktoppr", wp).Run(); err == nil {
			return nil
		}
	}
	// Fallback: System Events (bypasses Finder).
	script1 := fmt.Sprintf(`tell application "System Events" to set picture of every desktop to POSIX file %q`, wp)
	if err := exec.CommandContext(ctx, "osascript", "-e", script1).Run(); err == nil {
		return nil
	}
	// Last resort: Finder.
	script2 := fmt.Sprintf(`tell application "Finder" to set desktop picture to POSIX file %q`, wp)
	if err := exec.CommandContext(ctx, "osascript", "-e", script2).Run(); err == nil {
		return nil
	}
	return fmt.Errorf("hookWallpaper: all macOS setters failed for %s", wp)
}

func setWallpaperLinux(ctx context.Context, wp string) error {
	if _, err := exec.LookPath("swww"); err == nil {
		_ = exec.CommandContext(ctx, "swww", "query").Run()
		if err := exec.CommandContext(ctx, "swww", "img", wp,
			"--transition-type", "any").Run(); err == nil {
			return nil
		}
	}
	if _, err := exec.LookPath("hyprctl"); err == nil {
		_ = exec.CommandContext(ctx, "hyprctl", "hyprpaper", "reload", ","+wp).Run()
	}
	return nil // best effort; missing setter is fine
}
