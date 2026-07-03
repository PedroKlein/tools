package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func runSync() {
	agentDir := resolveAgentDir()

	profiles, err := discoverProfiles(agentDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(profiles) == 0 {
		fmt.Println("No profiles found in", filepath.Join(agentDir, "profiles"))
		return
	}

	for name, profile := range profiles {
		if err := syncProfile(agentDir, name, profile); err != nil {
			fmt.Fprintf(os.Stderr, "error syncing %q: %v\n", name, err)
			os.Exit(1)
		}
	}
}

func syncProfile(agentDir, name string, profile Profile) error {
	// Output directory is sibling to agent dir: ~/.pi/agent-<name>
	parentDir := filepath.Dir(agentDir)
	outDir := filepath.Join(parentDir, "agent-"+name)

	// Create if it doesn't exist (non-destructive — preserve existing content)
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	// 1. Merge settings (always overwrite — derived from base + overrides)
	if err := syncSettings(agentDir, outDir, name, profile); err != nil {
		return fmt.Errorf("syncing settings: %w", err)
	}

	// 2. Extensions: remove symlinks dir and recreate (cheap, just symlinks)
	extDir := filepath.Join(outDir, "extensions")
	_ = os.RemoveAll(extDir) // best-effort; extensions dir is recreated below

	if err := syncExtensions(agentDir, outDir, name, profile); err != nil {
		return fmt.Errorf("syncing extensions: %w", err)
	}

	// 3. Shared resources: ensure symlinks exist
	if err := syncResources(agentDir, outDir); err != nil {
		return fmt.Errorf("syncing resources: %w", err)
	}

	// 4. Ensure sessions dir exists (never touch contents)
	if err := os.MkdirAll(filepath.Join(outDir, "sessions"), 0o750); err != nil {
		return fmt.Errorf("creating sessions dir: %w", err)
	}

	extCount := len(profile.SharedExtensions)

	profileExtDir := filepath.Join(agentDir, "profiles", name, "extensions")
	if entries, err := os.ReadDir(profileExtDir); err == nil {
		extCount += len(entries)
	}

	fmt.Printf("✓ agent-%s/ synced (%d extensions, %d packages)\n", name, extCount, len(profile.Packages))

	return nil
}

func syncSettings(agentDir, outDir, _ string, profile Profile) error {
	// Read base settings
	baseSettingsPath := filepath.Join(agentDir, "settings.json")

	baseData, err := os.ReadFile(baseSettingsPath) //nolint:gosec // path derived from trusted agent dir
	if err != nil {
		return fmt.Errorf("reading base settings: %w", err)
	}

	var base map[string]any
	if parseErr := json.Unmarshal(baseData, &base); parseErr != nil {
		return fmt.Errorf("parsing base settings: %w", parseErr)
	}

	// Apply overrides
	if profile.SettingsOverrides != nil {
		base = deepMerge(base, profile.SettingsOverrides)
	}

	// Replace packages
	if profile.Packages != nil {
		pkgs := make([]any, len(profile.Packages))
		for i, p := range profile.Packages {
			pkgs[i] = p
		}

		base["packages"] = pkgs
	}

	// Set session dir
	base["sessionDir"] = filepath.Join(outDir, "sessions")

	// Write merged settings
	out, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}

	if err := os.WriteFile(filepath.Join(outDir, "settings.json"), out, 0o600); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}

	return nil
}

func syncExtensions(agentDir, outDir, name string, profile Profile) error {
	extDir := filepath.Join(outDir, "extensions")
	if err := os.MkdirAll(extDir, 0o750); err != nil {
		return fmt.Errorf("creating extensions dir: %w", err)
	}

	// Symlink shared extensions
	for _, ext := range profile.SharedExtensions {
		src := filepath.Join(agentDir, "extensions", ext)

		dst := filepath.Join(extDir, ext)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			return fmt.Errorf("shared extension %q not found at %s", ext, src)
		}

		if err := os.Symlink(src, dst); err != nil {
			return fmt.Errorf("symlinking extension %q: %w", ext, err)
		}
	}

	// Symlink profile-specific extensions
	profileExtDir := filepath.Join(agentDir, "profiles", name, "extensions")

	entries, err := os.ReadDir(profileExtDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no profile-specific extensions
		}

		return fmt.Errorf("reading profile extensions: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		src := filepath.Join(profileExtDir, entry.Name())

		dst := filepath.Join(extDir, entry.Name())
		if err := os.Symlink(src, dst); err != nil {
			return fmt.Errorf("symlinking profile extension %q: %w", entry.Name(), err)
		}
	}

	return nil
}

func syncResources(agentDir, outDir string) error {
	resources := []string{"models.json", "themes", "auth.json"}
	for _, res := range resources {
		src := filepath.Join(agentDir, res)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue // skip missing resources
		}

		dst := filepath.Join(outDir, res)
		// Remove existing symlink/file before recreating
		_ = os.Remove(dst) // best-effort cleanup before recreating symlink

		if err := os.Symlink(src, dst); err != nil {
			return fmt.Errorf("symlinking %s: %w", res, err)
		}
	}

	return nil
}
