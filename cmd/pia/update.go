package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
)

const piPackage = "@earendil-works/pi-coding-agent"

type updateOptions struct {
	extensionsOnly bool
	piVersion      string
}

var exactVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

func parseUpdateOptions(args []string) (updateOptions, error) {
	var options updateOptions

	for len(args) > 0 {
		switch args[0] {
		case "--extensions-only":
			options.extensionsOnly = true
			args = args[1:]
		case "--pi-version":
			if len(args) < 2 || !exactVersionPattern.MatchString(args[1]) {
				return updateOptions{}, errors.New("--pi-version requires an exact semantic version")
			}

			options.piVersion = args[1]
			args = args[2:]
		default:
			return updateOptions{}, fmt.Errorf("unknown update option %q", args[0])
		}
	}

	if options.extensionsOnly && options.piVersion != "" {
		return updateOptions{}, errors.New("--extensions-only and --pi-version cannot be combined")
	}

	return options, nil
}

func runUpdate(options updateOptions) {
	agentDir := resolveAgentDir()
	if err := updateProfiles(agentDir, options); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✓ All profiles updated")
}

func updateProfiles(agentDir string, options updateOptions) error {
	if err := syncProfiles(agentDir, nil); err != nil {
		return fmt.Errorf("syncing profiles before update: %w", err)
	}

	if options.piVersion != "" {
		npmPath, err := exec.LookPath("npm")
		if err != nil {
			return fmt.Errorf("finding npm in PATH: %w", err)
		}

		if err := runCommand(npmPath, "install", "--global", "--ignore-scripts", piPackage+"@"+options.piVersion); err != nil {
			return fmt.Errorf("installing Pi %s: %w", options.piVersion, err)
		}
	}

	piPath, err := exec.LookPath("pi")
	if err != nil {
		return fmt.Errorf("finding pi in PATH: %w", err)
	}

	if options.piVersion == "" && !options.extensionsOnly {
		if updateErr := runPiUpdate(piPath, agentDir); updateErr != nil {
			return fmt.Errorf("updating Pi: %w", updateErr)
		}
	}

	profiles, err := discoverProfiles(agentDir)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}

	sort.Strings(names)

	agentDirs := []string{agentDir}
	for _, name := range names {
		agentDirs = append(agentDirs, filepath.Join(filepath.Dir(agentDir), "agent-"+name))
	}

	for _, dir := range agentDirs {
		if updateErr := runPiExtensionsUpdate(piPath, dir); updateErr != nil {
			return fmt.Errorf("updating extensions in %s: %w", dir, updateErr)
		}
	}

	if err := syncProfiles(agentDir, nil); err != nil {
		return fmt.Errorf("syncing profiles after update: %w", err)
	}

	return nil
}

func runPiUpdate(piPath, agentDir string) error {
	return runPiCommand(piPath, agentDir, "update")
}

func runPiExtensionsUpdate(piPath, agentDir string) error {
	return runPiCommand(piPath, agentDir, "update", "--extensions")
}

func runPiCommand(piPath, agentDir string, args ...string) error {
	cmd := exec.Command(piPath, args...)

	cmd.Env = append(os.Environ(), "PI_CODING_AGENT_DIR="+agentDir)
	cmd.Stdout = os.Stdout

	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running pi %v: %w", args, err)
	}

	return nil
}

func runCommand(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout

	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running %s %v: %w", filepath.Base(path), args, err)
	}

	return nil
}
