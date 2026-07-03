package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Client connects to the q daemon and sends prompts.
type Client struct {
	socketPath string
}

// NewClient creates a client targeting the default socket.
func NewClient() *Client {
	return &Client{socketPath: SocketPath()}
}

// Prompt sends a prompt and streams text deltas to stdout.
// If resetSession is true, sends new_session before the prompt.
// If quiet is true, suppresses streaming output (collects silently).
// Returns the full response text.
//
//nolint:gocyclo // streaming loop with session reset requires multiple error paths
func (c *Client) Prompt(message string, resetSession, quiet bool) (string, error) {
	conn, err := c.connect()
	if err != nil {
		return "", err
	}

	defer func(cn net.Conn) { _ = cn.Close() }(conn)

	// Reset session if requested
	if resetSession {
		req := ClientRequest{Type: "new_session"}

		if encErr := Encode(conn, req); encErr != nil {
			return "", fmt.Errorf("sending new_session: %w", encErr)
		}

		// Read ack — but we need a new connection since daemon is one-request-per-connection
		_ = conn.Close()

		// Reconnect for the prompt
		conn, err = c.connect()
		if err != nil {
			return "", err
		}

		defer func(cn net.Conn) { _ = cn.Close() }(conn)
	}

	// Send prompt
	req := ClientRequest{Type: "prompt", Message: message}
	if err := Encode(conn, req); err != nil {
		return "", fmt.Errorf("sending prompt: %w", err)
	}

	// Stream response
	dec := NewDecoder(conn)

	var full strings.Builder

	for {
		event, err := dec.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return full.String(), nil
			}

			return full.String(), fmt.Errorf("reading response: %w", err)
		}

		if event.Type == "error" {
			return full.String(), fmt.Errorf("daemon error: %s", event.Error)
		}

		if event.IsTextDelta() {
			delta := event.AssistantMessageEvent.Delta
			if !quiet {
				fmt.Print(delta)
			}

			full.WriteString(delta)
		}

		if event.IsAgentEnd() {
			// Print final newline if response didn't end with one
			if s := full.String(); !quiet && s != "" && !strings.HasSuffix(s, "\n") {
				fmt.Println()
			}

			return full.String(), nil
		}
	}
}

// Ping checks if the daemon is alive.
func (c *Client) Ping() bool {
	return isSocketAlive(c.socketPath)
}

// dial connects to the socket without auto-start.
func (c *Client) dial() (net.Conn, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, time.Second)
	if err != nil {
		return nil, fmt.Errorf("dialing socket %s: %w", c.socketPath, err)
	}

	return conn, nil
}

// connect dials the daemon socket, auto-starting if needed.
func (c *Client) connect() (net.Conn, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, time.Second)
	if err == nil {
		return conn, nil
	}

	// Auto-start daemon
	if autoErr := c.autoStart(); autoErr != nil {
		return nil, fmt.Errorf("auto-starting daemon: %w", autoErr)
	}

	// Poll until socket is ready
	deadline := time.Now().Add(5 * time.Second)
	backoff := 50 * time.Millisecond

	for time.Now().Before(deadline) {
		time.Sleep(backoff)

		conn, err = net.DialTimeout("unix", c.socketPath, time.Second)
		if err == nil {
			return conn, nil
		}

		backoff = min(backoff*2, 500*time.Millisecond)
	}

	return nil, fmt.Errorf("daemon did not start within timeout (socket: %s)", c.socketPath)
}

// autoStart forks the q binary with "start" in the background.
func (c *Client) autoStart() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	cmd := exec.Command(exe, "start")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}

	// Detach — don't wait
	_ = cmd.Process.Release()

	return nil
}

// ReadStdinContext reads piped stdin (non-interactive) and returns it.
// Returns empty string if stdin is a terminal.
func ReadStdinContext() string {
	info, err := os.Stdin.Stat()
	if err != nil {
		return ""
	}

	// Check if stdin has data (pipe or redirect)
	if info.Mode()&os.ModeCharDevice != 0 {
		return "" // interactive terminal, skip
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return ""
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return ""
	}

	return content
}

// BuildPromptWithContext combines stdin context with the user prompt.
func BuildPromptWithContext(stdinContext, prompt string) string {
	if stdinContext == "" {
		return prompt
	}

	return fmt.Sprintf("<context>\n%s\n</context>\n\n%s", stdinContext, prompt)
}
