package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func runUpdate() {
	agentDir := resolveAgentDir()

	// Find pi binary
	piPath, err := exec.LookPath("pi")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: pi not found in PATH\n")
		os.Exit(1)
	}

	// 1. Update default agent dir
	fmt.Println("Updating default profile...")

	if updateErr := runPiUpdate(piPath, agentDir); updateErr != nil {
		fmt.Fprintf(os.Stderr, "error updating default: %v\n", updateErr)
		os.Exit(1)
	}

	// 2. Update each synced profile dir
	profiles, err := discoverProfiles(agentDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	parentDir := filepath.Dir(agentDir)
	for name := range profiles {
		syncedDir := filepath.Join(parentDir, "agent-"+name)
		if _, err := os.Stat(syncedDir); os.IsNotExist(err) {
			fmt.Printf("⚠ agent-%s/ not synced, skipping\n", name)
			continue
		}

		fmt.Printf("Updating %s...\n", name)

		if err := runPiUpdate(piPath, syncedDir); err != nil {
			fmt.Fprintf(os.Stderr, "error updating %s: %v\n", name, err)
			os.Exit(1)
		}
	}

	// 3. Re-sync profiles to pick up any new resources
	fmt.Println("\nRe-syncing profiles...")
	runSync()

	fmt.Println("\n✓ All profiles updated")
}

func runPiUpdate(piPath, agentDir string) error {
	cmd := exec.Command(piPath, "update", "--all")

	cmd.Env = append(os.Environ(), "PI_CODING_AGENT_DIR="+agentDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running pi update: %w", err)
	}

	return nil
}
