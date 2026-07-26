package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroKlein/tools/cmd/themes/internal/reload"
	xdgstate "github.com/PedroKlein/tools/cmd/themes/internal/state"
)

// WallpaperList returns all wallpaper file paths for the currently active theme.
func WallpaperList(theme string) ([]string, error) {
	dir := filepath.Join(themeDir(theme), "backgrounds")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") ||
			strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".webp") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

var applyWallpaperHook = func(themeRoot string) error {
	liveApply := false
	skip := map[string]bool{}
	for _, h := range reload.Registry() {
		if h.Name != "wallpaper" {
			skip[h.Name] = true
		}
	}
	results := reload.RunAll(context.Background(), themeRoot, reload.Options{
		SkipHooks:     skip,
		LiveApply:     &liveApply,
		SkipUserHooks: true,
		Verbose:       os.Getenv("THEME_VERBOSE") != "",
		Stderr:        os.Stderr,
	})
	var errs []error
	for _, result := range results {
		if result.Err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", result.Name, result.Err))
		}
	}
	return errors.Join(errs...)
}

var previewWallpaperHook = func(themeRoot string) error {
	ctx, cancel := context.WithTimeout(context.Background(), wallpaperPreviewTimeout)
	defer cancel()
	return reload.PreviewWallpaper(ctx, themeRoot)
}

func PreviewWallpaper(theme string) error {
	if theme == "" {
		return nil
	}
	return previewWallpaperHook(themeDir(theme))
}

// SetWallpaper writes the wallpaper path into XDG state and applies the Go
// wallpaper hook. The old shell hook was deleted when in-repo hooks moved
// into cmd/themes/internal/reload.
func SetWallpaper(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if !fileExists(abs) {
		return fmt.Errorf("wallpaper not found: %s", abs)
	}

	s, err := xdgstate.Load()
	if err != nil {
		return err
	}
	s.Wallpaper = abs
	if s.Theme != "" {
		if s.WallpaperByTheme == nil {
			s.WallpaperByTheme = map[string]string{}
		}
		s.WallpaperByTheme[s.Theme] = abs
	}
	if err := xdgstate.Save(*s); err != nil {
		return err
	}

	themeRoot := ""
	if s.Theme != "" {
		themeRoot = themeDir(s.Theme)
	} else if target := xdgstate.CurrentTarget(); target != "" {
		themeRoot = target
		if filepath.Base(themeRoot) == "derived" {
			themeRoot = filepath.Dir(themeRoot)
		}
	}
	if themeRoot == "" {
		return nil
	}
	return applyWallpaperHook(themeRoot)
}

// RandomWallpaperFor returns a random wallpaper path for a theme, or "" if none.
func RandomWallpaperFor(theme string) string {
	list, err := WallpaperList(theme)
	if err != nil || len(list) == 0 {
		return ""
	}
	return list[rand.IntN(len(list))]
}

// CycleWallpaper picks the next wallpaper after the currently-active one
// for the given theme and applies it. Wraps around at the end of the list.
// Silent no-op if the theme has zero or one wallpapers.
func CycleWallpaper(theme string) error {
	list, err := WallpaperList(theme)
	if err != nil {
		return err
	}
	if len(list) < 2 {
		return nil
	}
	s, err := xdgstate.Load()
	if err != nil {
		return err
	}
	current := ""
	if s.WallpaperByTheme != nil {
		current = s.WallpaperByTheme[theme]
	}
	if current == "" {
		current = s.Wallpaper
	}
	// Find current index; if not in list (state stale), start at -1 so next==0.
	idx := -1
	for i, p := range list {
		if p == current {
			idx = i
			break
		}
	}
	next := list[(idx+1)%len(list)]
	return SetWallpaper(next)
}
