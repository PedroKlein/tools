package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// OpenResult is the JSON output for repos open.
type OpenResult struct {
	Workspace string `json:"workspace"`
	Path      string `json:"path"`
	Created   bool   `json:"created"`
	Focused   bool   `json:"focused"`
}

type herdrWorkspace struct {
	ID    string `json:"workspace_id"`
	Label string `json:"label"`
}

func runOpen(args []string) {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	repoPath, relPath, err := resolveRepo(query)
	if err != nil {
		openError(err.Error())
	}

	if _, lookErr := exec.LookPath("herdr"); lookErr != nil {
		openError("Herdr not found in PATH")
	}

	workspaceLabel := relPath
	worktreeDir := defaultWorktreePath(repoPath)

	workspaceID, created, err := openHerdrWorkspace(worktreeDir, workspaceLabel)
	if err != nil {
		openError(err.Error())
	}

	if jsonOutput {
		writeJSON(OpenResult{
			Workspace: workspaceID,
			Path:      worktreeDir,
			Created:   created,
			Focused:   true,
		})

		return
	}

	action := "focused"
	if created {
		action = "opened"
	}

	fmt.Printf("%s %s (workspace: %s)\n", action, relPath, workspaceLabel)
}

func openHerdrWorkspace(worktreeDir, workspaceLabel string) (workspaceID string, created bool, err error) {
	out, err := runHerdr(worktreeDir, "workspace", "list")
	if err != nil {
		return "", false, fmt.Errorf("listing Herdr workspaces: %w: %s", err, strings.TrimSpace(string(out)))
	}

	var listResponse struct {
		Result struct {
			Workspaces []herdrWorkspace `json:"workspaces"`
		} `json:"result"`
	}

	if decodeErr := json.Unmarshal(out, &listResponse); decodeErr != nil {
		return "", false, fmt.Errorf("decoding Herdr workspace list: %w", decodeErr)
	}

	for _, workspace := range listResponse.Result.Workspaces {
		if workspace.Label != workspaceLabel {
			continue
		}

		focusOutput, focusErr := runHerdr(worktreeDir, "workspace", "focus", workspace.ID)
		if focusErr != nil {
			return "", false, fmt.Errorf("focusing Herdr workspace: %w: %s", focusErr, strings.TrimSpace(string(focusOutput)))
		}

		return workspace.ID, false, nil
	}

	out, err = runHerdr(worktreeDir, "workspace", "create", "--cwd", worktreeDir, "--label", workspaceLabel, "--focus")
	if err != nil {
		return "", false, fmt.Errorf("creating Herdr workspace: %w: %s", err, strings.TrimSpace(string(out)))
	}

	var createResponse struct {
		Result struct {
			Workspace herdrWorkspace `json:"workspace"`
		} `json:"result"`
	}

	if decodeErr := json.Unmarshal(out, &createResponse); decodeErr != nil {
		return "", false, fmt.Errorf("decoding Herdr workspace create: %w", decodeErr)
	}

	if createResponse.Result.Workspace.ID == "" {
		return "", false, errors.New("decoding Herdr workspace create: response has no workspace id")
	}

	return createResponse.Result.Workspace.ID, true, nil
}

func runHerdr(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("herdr", args...)
	cmd.Dir = dir

	return cmd.CombinedOutput() //nolint:wrapcheck // caller adds workspace operation context
}

func openError(message string) {
	if jsonOutput {
		writeJSONError(message, ExitError)
	}

	fmt.Fprintf(os.Stderr, "error: %s\n", message)
	os.Exit(ExitError)
}
