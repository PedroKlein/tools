package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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

	piPath, err := exec.LookPath("pi")
	if err != nil {
		issues = append(issues, "✗ pi not found in PATH")
	} else {
		versionOutput, versionErr := exec.Command(piPath, "--version").CombinedOutput()
		if versionErr != nil {
			issues = append(issues, fmt.Sprintf("✗ reading Pi version: %v", versionErr))
		} else {
			fmt.Printf("✓ Pi version: %s\n", strings.TrimSpace(string(versionOutput)))
		}
	}

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

	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}

	sort.Strings(names)

	agentDirs := make([]string, 1, len(names)+1)

	agentDirs[0] = agentDir
	for _, name := range names {
		agentDirs = append(agentDirs, filepath.Join(filepath.Dir(agentDir), "agent-"+name))
	}

	if piPath != "" {
		for _, dir := range agentDirs {
			output, listErr := runOfflinePiList(piPath, dir)
			if listErr != nil {
				issues = append(issues, fmt.Sprintf("✗ pi list failed for %s: %v", dir, listErr))
				continue
			}

			lowerOutput := strings.ToLower(output)
			if strings.Contains(lowerOutput, "warning") || strings.Contains(lowerOutput, "error") {
				issues = append(issues, fmt.Sprintf("✗ pi list warnings for %s: %s", dir, strings.TrimSpace(output)))
			}
		}
	}

	for _, name := range names {
		profile := profiles[name]
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

			expected, renderErr := renderSettings(agentDir, profile)

			actual, readErr := os.ReadFile(filepath.Join(syncedDir, "settings.json")) //nolint:gosec // path is the managed profile directory
			if renderErr != nil {
				issues = append(issues, fmt.Sprintf("  ✗ rendering settings: %v", renderErr))
			} else if readErr != nil {
				issues = append(issues, fmt.Sprintf("  ✗ reading generated settings: %v", readErr))
			} else if !bytes.Equal(actual, expected) {
				issues = append(issues, "  ✗ settings drift (run: pia sync)")
			}

			for _, resource := range []string{"mcp.json", "web-search.json"} {
				expectedTarget := filepath.Join(agentDir, "profiles", name, resource)
				if _, statErr := os.Stat(expectedTarget); os.IsNotExist(statErr) && resource == "web-search.json" {
					expectedTarget = filepath.Join(agentDir, resource)
				}

				if _, statErr := os.Stat(expectedTarget); os.IsNotExist(statErr) {
					continue
				}

				actualTarget, linkErr := os.Readlink(filepath.Join(syncedDir, resource))
				if linkErr != nil || actualTarget != expectedTarget {
					issues = append(issues, fmt.Sprintf("  ✗ %s link drift (run: pia sync)", resource))
				}
			}
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

func runOfflinePiList(piPath, agentDir string) (string, error) {
	cmd := exec.Command(piPath, "list", "--no-approve")

	cmd.Env = append(os.Environ(), "PI_CODING_AGENT_DIR="+agentDir, "PI_OFFLINE=1")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("running pi list: %w", err)
	}

	return string(output), nil
}
