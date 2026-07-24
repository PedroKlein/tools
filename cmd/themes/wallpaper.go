package main

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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

// SetWallpaper writes the wallpaper path into state and invokes hooks/wallpaper.sh.
func SetWallpaper(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if !fileExists(abs) {
		return fmt.Errorf("wallpaper not found: %s", abs)
	}
	s, err := LoadState()
	if err != nil {
		return err
	}
	s.Wallpaper = abs
	if s.Theme != "" {
		s.WallpaperByTheme[s.Theme] = abs
	}
	if err := s.Save(); err != nil {
		return err
	}
	// Invoke wallpaper hook with the path.
	hook := hookScript("wallpaper.sh")
	if !fileExists(hook) {
		return nil
	}
	current := currentPath()
	cmd := exec.Command(hook, current, abs)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RandomWallpaperFor returns a random wallpaper path for a theme, or "" if none.
func RandomWallpaperFor(theme string) string {
	list, err := WallpaperList(theme)
	if err != nil || len(list) == 0 {
		return ""
	}
	//nolint:gosec // theme picker doesn't need crypto RNG
	return list[rand.Intn(len(list))]
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
	s, err := LoadState()
	if err != nil {
		return err
	}
	current := s.WallpaperByTheme[theme]
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
