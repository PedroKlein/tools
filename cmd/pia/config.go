package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Profile defines a pi profile configuration.
type Profile struct {
	Description       string         `json:"description"`
	Model             string         `json:"model,omitempty"`
	Thinking          string         `json:"thinking,omitempty"`
	SharedExtensions  []string       `json:"sharedExtensions"`
	Packages          []string       `json:"packages,omitempty"`
	Skills            []string       `json:"skills,omitempty"`
	SettingsOverrides map[string]any `json:"settingsOverrides,omitempty"`
	Flags             []string       `json:"flags,omitempty"`
}

// resolveAgentDir returns the base Pi agent directory.
// Uses PIA_AGENT_DIR env var if set (for testing), otherwise ~/.pi/agent.
func resolveAgentDir() string {
	if dir := os.Getenv("PIA_AGENT_DIR"); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}

	return filepath.Join(home, ".pi", "agent")
}

// loadProfile reads and parses a profile.json file.
func loadProfile(path string) (Profile, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from profile.json location, caller-controlled
	if err != nil {
		return Profile{}, fmt.Errorf("reading profile: %w", err)
	}

	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("parsing profile: %w", err)
	}

	return p, nil
}

// discoverProfiles finds all profiles under agentDir/profiles/.
func discoverProfiles(agentDir string) (map[string]Profile, error) {
	profilesDir := filepath.Join(agentDir, "profiles")

	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("reading profiles directory: %w", err)
	}

	profiles := make(map[string]Profile)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		profilePath := filepath.Join(profilesDir, entry.Name(), "profile.json")

		p, err := loadProfile(profilePath)
		if err != nil {
			continue // skip profiles that can't be loaded
		}

		profiles[entry.Name()] = p
	}

	return profiles, nil
}
