package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const profilesSubcmd = "profiles"

// ParsedArgs holds the result of CLI argument parsing.
type ParsedArgs struct {
	Profile    string
	Copy       bool
	Run        bool
	New        bool
	Prompt     string
	Subcommand string // "start", "stop", "status", "profiles", or "" for prompt mode
	NamesOnly  bool   // for "profiles --names"
}

// ParseArgs parses CLI arguments into structured form.
func ParseArgs(args []string) ParsedArgs {
	var p ParsedArgs

	if len(args) == 0 {
		return p
	}

	// Check for subcommand
	switch args[0] {
	case "start", "stop", "status", "reset":
		p.Subcommand = args[0]
		return p
	case profilesSubcmd:
		p.Subcommand = profilesSubcmd

		for _, a := range args[1:] {
			if a == "--names" {
				p.NamesOnly = true
			}
		}

		return p
	}

	// Parse flags and collect prompt words
	var promptParts []string

	i := 0
	for i < len(args) {
		switch args[i] {
		case "-p", "--profile":
			if i+1 < len(args) {
				p.Profile = args[i+1]
				i += 2
			} else {
				i++
			}
		case "-c", "--copy":
			p.Copy = true
			i++
		case "--run", "-r":
			p.Run = true
			i++
		case "--new":
			p.New = true
			i++
		default:
			promptParts = append(promptParts, args[i])
			i++
		}
	}

	p.Prompt = strings.Join(promptParts, " ")

	return p
}

// CopyToClipboard copies text to the system clipboard.
func CopyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}

	cmd.Stdin = strings.NewReader(text)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clipboard copy: %w", err)
	}

	return nil
}

// ConfirmRun shows the command and asks for confirmation.
// Reads from /dev/tty to work even with piped stdin.
func ConfirmRun(command string) bool {
	fmt.Fprintf(os.Stderr, "\n\033[1m%s\033[0m\n", command)
	fmt.Fprintf(os.Stderr, "Run? [y/N] ")

	// Open /dev/tty directly so confirmation works with piped stdin
	tty, err := os.Open("/dev/tty")
	if err != nil {
		// Fallback to stderr (won't work for input, but don't crash)
		return false
	}

	defer func() { _ = tty.Close() }()

	scanner := bufio.NewScanner(tty)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "y" || answer == "yes"
	}

	return false
}

// ExecuteCommand runs a shell command via sh -c.
func ExecuteCommand(command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("executing command: %w", err)
	}

	return nil
}
