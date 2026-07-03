package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// HooksConfig holds all lifecycle hooks keyed by event name.
type HooksConfig struct {
	PostClone []string `json:"post-clone,omitempty"`
	PostSync  []string `json:"post-sync,omitempty"`
	PostTidy  []string `json:"post-tidy,omitempty"`
}

// validEvents lists the supported hook event names.
var validEvents = []string{"post-clone", "post-sync", "post-tidy"}

// hooksConfigPath returns the path to the hooks config file.
func hooksConfigPath() string {
	return filepath.Join(homeDir(), ".config", "repos", "hooks.json")
}

// loadHooks reads the hooks config from disk. Returns empty config if file
// doesn't exist or cannot be parsed.
func loadHooks() HooksConfig {
	var cfg HooksConfig

	data, err := os.ReadFile(hooksConfigPath())
	if err != nil {
		return cfg
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg // best-effort; bad JSON → empty config
	}

	return cfg
}

// saveHooks writes the hooks config to disk, creating parent dirs as needed.
func saveHooks(cfg HooksConfig) error {
	path := hooksConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling hooks config: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing hooks config: %w", err)
	}

	return nil
}

// getHooksForEvent returns the hook commands for a given event name.
func getHooksForEvent(cfg HooksConfig, event string) []string {
	switch event {
	case "post-clone":
		return cfg.PostClone
	case "post-sync":
		return cfg.PostSync
	case "post-tidy":
		return cfg.PostTidy
	}

	return nil
}

// setHooksForEvent replaces the hook list for the given event.
func setHooksForEvent(cfg *HooksConfig, event string, hooks []string) {
	switch event {
	case "post-clone":
		cfg.PostClone = hooks
	case "post-sync":
		cfg.PostSync = hooks
	case "post-tidy":
		cfg.PostTidy = hooks
	}
}

// isValidEvent returns true if event is a known hook event.
func isValidEvent(event string) bool {
	return slices.Contains(validEvents, event)
}

// runHookCmd dispatches `repos hook <list|add|rm> [args...]`.
func runHookCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: repos hook <list|add|rm> [event] [command]")
		os.Exit(ExitError)
	}

	action := args[0]
	rest := args[1:]

	switch action {
	case "list":
		hookList()
	case "add":
		if len(rest) < 2 {
			fmt.Fprintln(os.Stderr, "usage: repos hook add <event> <command>")
			os.Exit(ExitError)
		}

		hookAdd(rest[0], strings.Join(rest[1:], " "))
	case "rm", "remove":
		if len(rest) < 2 {
			fmt.Fprintln(os.Stderr, "usage: repos hook rm <event> <command>")
			os.Exit(ExitError)
		}

		hookRemove(rest[0], strings.Join(rest[1:], " "))
	default:
		fmt.Fprintf(os.Stderr, "error: unknown hook action %q\n", action)
		os.Exit(ExitError)
	}
}

func hookList() {
	cfg := loadHooks()

	if jsonOutput {
		writeJSON(cfg)
		return
	}

	for _, event := range validEvents {
		hooks := getHooksForEvent(cfg, event)
		if len(hooks) == 0 {
			fmt.Printf("%s: (none)\n", event)
		} else {
			fmt.Printf("%s:\n", event)

			for _, h := range hooks {
				fmt.Printf("  %s\n", h)
			}
		}
	}
}

func hookAdd(event, command string) {
	if !isValidEvent(event) {
		fmt.Fprintf(os.Stderr, "error: invalid event %q (valid: %s)\n", event, strings.Join(validEvents, ", "))
		os.Exit(ExitError)
	}

	cfg := loadHooks()
	hooks := getHooksForEvent(cfg, event)

	// Deduplicate
	if slices.Contains(hooks, command) {
		fmt.Printf("hook already exists for %s: %s\n", event, command)
		return
	}

	hooks = append(hooks, command)
	setHooksForEvent(&cfg, event, hooks)

	if err := saveHooks(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error saving hooks: %v\n", err)
		os.Exit(ExitError)
	}

	fmt.Printf("added to %s: %s\n", event, command)
}

func hookRemove(event, command string) {
	if !isValidEvent(event) {
		fmt.Fprintf(os.Stderr, "error: invalid event %q (valid: %s)\n", event, strings.Join(validEvents, ", "))
		os.Exit(ExitError)
	}

	cfg := loadHooks()
	hooks := getHooksForEvent(cfg, event)

	found := false

	var filtered []string

	for _, h := range hooks {
		if h == command {
			found = true
		} else {
			filtered = append(filtered, h)
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "hook not found for %s: %s\n", event, command)
		os.Exit(ExitNotFound)
	}

	setHooksForEvent(&cfg, event, filtered)

	if err := saveHooks(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error saving hooks: %v\n", err)
		os.Exit(ExitError)
	}

	fmt.Printf("removed from %s: %s\n", event, command)
}

// HookRepoInfo carries per-repo metadata exposed to hook commands as env vars.
type HookRepoInfo struct {
	Path   string // absolute path to repo root
	Name   string // repo name (last path component)
	Host   string
	Owner  string
	Branch string // default branch
}

// runHooksForEvent executes all hooks registered for event against each repo.
// Hooks are run via sh -c so they can use pipes, env vars, and shell builtins.
// Failures are printed to stderr but never abort the calling operation.
func runHooksForEvent(event string, repos []HookRepoInfo) {
	cfg := loadHooks()

	hooks := getHooksForEvent(cfg, event)
	if len(hooks) == 0 {
		return
	}

	for _, repo := range repos {
		for _, hook := range hooks {
			cmd := exec.Command("sh", "-c", hook)

			cmd.Env = append(os.Environ(),
				"REPO_PATH="+repo.Path,
				"REPO_NAME="+repo.Name,
				"REPO_HOST="+repo.Host,
				"REPO_OWNER="+repo.Owner,
				"REPO_BRANCH="+repo.Branch,
			)
			cmd.Stdout = os.Stdout

			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: hook %q for %s failed: %v\n", hook, repo.Name, err)
			}
		}
	}
}
