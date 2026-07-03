package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	if len(args) < 2 {
		printUsage()
		return 1
	}

	switch args[1] {
	case "-h", "--help", "help":
		printUsage()
		return 0
	case "-v", "--version":
		fmt.Println("q 0.1.0")
		return 0
	default:
		parsed := ParseArgs(args[1:])
		switch parsed.Subcommand {
		case "start":
			return cmdStart()
		case "stop":
			return cmdStop()
		case "status":
			return cmdStatus()
		case profilesSubcmd:
			return cmdProfiles(parsed.NamesOnly)
		case "reset":
			return cmdReset()
		default:
			return cmdPrompt(parsed)
		}
	}
}

func cmdStart() int {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	d := NewDaemon(cfg)
	if err := d.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

func cmdStop() int {
	pidPath := PIDPath()

	//nolint:gosec // pidPath is a system-controlled path from PIDPath()
	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon not running (no pid file)\n")
		return 1
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid pid file: %v\n", err)

		_ = os.Remove(pidPath)

		return 1
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "process %d not found\n", pid)

		_ = os.Remove(pidPath)
		_ = os.Remove(SocketPath())

		return 1
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "failed to stop daemon: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stderr, "daemon stopped (pid %d)\n", pid)

	return 0
}

func cmdStatus() int {
	client := NewClient()
	if client.Ping() {
		pidPath := PIDPath()

		//nolint:gosec // pidPath is a system-controlled path from PIDPath()
		data, _ := os.ReadFile(pidPath)
		pid := strings.TrimSpace(string(data))
		fmt.Printf("running (pid %s, socket %s)\n", pid, SocketPath())

		return 0
	}

	fmt.Println("stopped")

	return 1
}

func cmdProfiles(namesOnly bool) int {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}

	sort.Strings(names)

	if namesOnly {
		for _, name := range names {
			fmt.Println(name)
		}

		return 0
	}

	for _, name := range names {
		p := cfg.Profiles[name]

		marker := " "
		if name == cfg.DefaultProfile {
			marker = "*"
		}

		model := p.Model
		if model == "" {
			model = cfg.Model
		}

		desc := p.Instruction
		if desc == "" {
			desc = "(auto-detect format)"
		}

		fmt.Printf("%s %-10s  %-30s  %s\n", marker, name, model, truncate(desc, 50))
	}

	return 0
}

//nolint:gocyclo // command handler requires branching for all prompt flags and modes
func cmdPrompt(p ParsedArgs) int {
	if p.Prompt == "" {
		stdinCtx := ReadStdinContext()
		if stdinCtx == "" {
			printUsage()
			return 1
		}

		p.Prompt = stdinCtx
	} else {
		stdinCtx := ReadStdinContext()
		if stdinCtx != "" {
			p.Prompt = BuildPromptWithContext(stdinCtx, p.Prompt)
		}
	}

	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Resolve profile for validation
	_, err = ResolveProfile(cfg, p.Profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Prepend per-request instructions
	var prefix string

	// Profile instruction (e.g. "[Respond with a single shell command only...]")
	if instr := GetInstruction(cfg, p.Profile); instr != "" {
		prefix = instr
	}

	// --run instruction overrides/appends
	if p.Run {
		prefix = RunInstruction
	}

	if prefix != "" {
		p.Prompt = prefix + "\n\n" + p.Prompt
	}

	// Use autoCopy from config if not set via flag
	if cfg.AutoCopy && !p.Copy {
		p.Copy = true
	}

	// When --run is active, suppress streaming (show only in confirmation)
	quiet := p.Run

	client := NewClient()

	response, err := client.Prompt(p.Prompt, p.New, quiet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Copy to clipboard
	if p.Copy {
		if err := CopyToClipboard(response); err != nil {
			fmt.Fprintf(os.Stderr, "warning: clipboard copy failed: %v\n", err)
		}
	}

	// Run mode
	if p.Run {
		cmd := strings.TrimSpace(response)
		if cmd == "" {
			fmt.Fprintf(os.Stderr, "no command to run\n")
			return 1
		}

		if ConfirmRun(cmd) {
			if err := ExecuteCommand(cmd); err != nil {
				fmt.Fprintf(os.Stderr, "command failed: %v\n", err)
				return 1
			}
		}
	}

	return 0
}

func cmdReset() int {
	client := NewClient()
	if !client.Ping() {
		fmt.Fprintf(os.Stderr, "daemon not running\n")
		return 1
	}

	conn, err := client.dial()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	defer func() { _ = conn.Close() }()

	req := ClientRequest{Type: "new_session"}

	if encErr := Encode(conn, req); encErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", encErr)
		return 1
	}

	dec := NewDecoder(conn)

	event, err := dec.Next()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if event.Type == "error" {
		fmt.Fprintf(os.Stderr, "error: %s\n", event.Error)
		return 1
	}

	fmt.Fprintf(os.Stderr, "session reset\n")

	return 0
}

func truncate(s string, n int) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}

	if len(s) > n {
		return s[:n-3] + "..."
	}

	return s
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `q — Quick questions via persistent Pi daemon

Usage:
  q [flags] <prompt>            Ask a question (auto-starts daemon)
  q start                       Pre-warm the daemon
  q stop                        Stop the daemon
  q reset                       Clear session (keep daemon running)
  q status                      Show daemon status
  q profiles                    List available profiles
  q profiles --names            List profile names only (for completion)

Flags:
  -p, --profile <name>   Use a specific profile (default: from config)
  -c, --copy             Copy response to clipboard
  --new                  Reset session before prompting
  --run, -r              Execute response as a command (with confirmation)

Examples:
  q "how to find files larger than 100MB"
  q -p explain "what is a monad"
  q -p think "debug this error"
  echo "error log" | q "explain this"
  q --run "delete all .DS_Store files"
  q -c "tar extract syntax"
`)
}
