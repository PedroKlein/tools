package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func runList() {
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

	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		p := profiles[name]
		fmt.Printf("  %-12s %s\n", name, p.Description)
	}
}

func runShow(name string) {
	agentDir := resolveAgentDir()
	profilePath := filepath.Join(agentDir, "profiles", name, "profile.json")

	profile, err := loadProfile(profilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: profile %q not found\n", name)
		os.Exit(1)
	}

	fmt.Printf("Profile: %s\n", name)
	fmt.Printf("Description: %s\n", profile.Description)

	if profile.Model != "" {
		fmt.Printf("Model: %s\n", profile.Model)
	}

	if profile.Thinking != "" {
		fmt.Printf("Thinking: %s\n", profile.Thinking)
	}

	if len(profile.SharedExtensions) > 0 {
		fmt.Printf("Extensions: %s\n", strings.Join(profile.SharedExtensions, ", "))
	}

	if len(profile.Packages) > 0 {
		fmt.Printf("Packages: %s\n", strings.Join(profile.Packages, ", "))
	}

	if len(profile.Skills) > 0 {
		fmt.Printf("Skills: %s\n", strings.Join(profile.Skills, ", "))
	}

	if len(profile.Flags) > 0 {
		fmt.Printf("Flags: %s\n", strings.Join(profile.Flags, " "))
	}
}

func runDoctor() {
	agentDir := resolveAgentDir()

	var issues []string

	// Check base settings
	settingsPath := filepath.Join(agentDir, "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		issues = append(issues, "✗ base settings.json not found: "+settingsPath)
	} else {
		fmt.Printf("✓ base settings.json: %s\n", settingsPath)
	}

	profiles, err := discoverProfiles(agentDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(profiles) == 0 {
		fmt.Println("✗ no profiles found")
		os.Exit(1)
	}

	for name, profile := range profiles {
		fmt.Printf("\n— %s\n", name)

		// Check shared extensions exist
		for _, ext := range profile.SharedExtensions {
			extPath := filepath.Join(agentDir, "extensions", ext)
			if _, err := os.Stat(extPath); os.IsNotExist(err) {
				issues = append(issues, fmt.Sprintf("  ✗ extension %q not found: %s", ext, extPath))
			} else {
				fmt.Printf("  ✓ extension: %s\n", ext)
			}
		}

		// Check synced dir
		parentDir := filepath.Dir(agentDir)

		syncedDir := filepath.Join(parentDir, "agent-"+name)
		if _, err := os.Stat(syncedDir); os.IsNotExist(err) {
			issues = append(issues, "  ✗ not synced (run: pia sync)")
		} else {
			fmt.Printf("  ✓ synced: %s\n", syncedDir)
		}
	}

	if len(issues) > 0 {
		fmt.Println("\nIssues:")

		for _, issue := range issues {
			fmt.Println(issue)
		}

		os.Exit(1)
	}

	fmt.Println("\n✓ all checks passed")
}
