package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}

		return home
	}

	if !strings.HasPrefix(path, "~/") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return filepath.Join(home, path[2:])
}

// buildArgs constructs the pi CLI arguments from a profile.
func buildArgs(profile Profile, extraArgs []string) []string {
	var args []string

	if profile.Model != "" {
		args = append(args, "--model", profile.Model)
	}

	if profile.Thinking != "" {
		args = append(args, "--thinking", profile.Thinking)
	}

	for _, skill := range profile.Skills {
		args = append(args, "--skill", expandTilde(skill))
	}
	// Expand tilde in flag values (path arguments following flags like
	// --prompt-template, --append-system-prompt, --skill, etc.)
	for _, flag := range profile.Flags {
		args = append(args, expandTilde(flag))
	}

	args = append(args, extraArgs...)

	return args
}

func launchProfile(name string, extraArgs []string) {
	agentDir := resolveAgentDir()

	// Load profile to get CLI flags
	profilePath := filepath.Join(agentDir, "profiles", name, "profile.json")

	profile, err := loadProfile(profilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: profile %q not found — check profiles in %s\n", name, filepath.Join(agentDir, "profiles"))
		os.Exit(1)
	}

	// Check synced directory exists
	parentDir := filepath.Dir(agentDir)

	syncedDir := filepath.Join(parentDir, "agent-"+name)
	if _, statErr := os.Stat(syncedDir); os.IsNotExist(statErr) { //nolint:gosec // syncedDir is a computed path within the user's home directory
		fmt.Fprintf(os.Stderr, "error: profile %q not synced — run: pia sync\n", name)
		os.Exit(1)
	}

	// Find pi binary
	piPath, err := exec.LookPath("pi")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: pi not found in PATH\n")
		os.Exit(1)
	}

	// Build args and exec
	piArgs := append([]string{"pi"}, buildArgs(profile, extraArgs)...)
	env := os.Environ()
	env = append(env, "PI_CODING_AGENT_DIR="+syncedDir)

	// Replace process with pi
	if err := syscall.Exec(piPath, piArgs, env); err != nil { //nolint:gosec // piPath comes from exec.LookPath; intentional process replacement
		fmt.Fprintf(os.Stderr, "error: exec pi: %v\n", err)
		os.Exit(1)
	}
}
