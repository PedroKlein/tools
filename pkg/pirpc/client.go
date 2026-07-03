package pirpc

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// PromptOnce connects to the q daemon socket, sends a prompt, and returns the
// full response text. It does not auto-start the daemon — returns an error if
// the socket is unavailable.
func PromptOnce(message string) (string, error) {
	return promptOnce(message, false)
}

// PromptFresh sends a new_session reset before the prompt, ensuring a clean
// context with no prior conversation history. Use for one-shot tasks like
// parsing or classification where prior context would pollute results.
func PromptFresh(message string) (string, error) {
	return promptOnce(message, true)
}

func promptOnce(message string, fresh bool) (string, error) {
	conn, err := net.DialTimeout("unix", SocketPath(), 2*time.Second)
	if err != nil {
		return "", fmt.Errorf("connecting to q daemon: %w", err)
	}

	defer func() { _ = conn.Close() }() // best-effort; connection errors after response are non-critical

	// Set a generous deadline for LLM responses
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return "", fmt.Errorf("setting connection deadline: %w", err)
	}

	// Reset session if fresh mode requested
	if fresh {
		resetReq := ClientRequest{Type: "new_session"}
		if err := Encode(conn, resetReq); err != nil {
			return "", fmt.Errorf("sending new_session: %w", err)
		}
		// Read the ack
		ackDec := NewDecoder(conn)
		if _, err := ackDec.Next(); err != nil {
			return "", fmt.Errorf("new_session ack: %w", err)
		}
	}

	// Send prompt
	req := ClientRequest{Type: "prompt", Message: message}
	if err := Encode(conn, req); err != nil {
		return "", fmt.Errorf("sending prompt: %w", err)
	}

	// Read full response
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
			return full.String(), fmt.Errorf("daemon: %s", event.Error)
		}

		if event.IsTextDelta() {
			full.WriteString(event.AssistantMessageEvent.Delta)
		}

		if event.IsAgentEnd() {
			return full.String(), nil
		}
	}
}

// IsAvailable checks whether the q daemon socket is reachable.
func IsAvailable() bool {
	conn, err := net.DialTimeout("unix", SocketPath(), 500*time.Millisecond)
	if err != nil {
		return false
	}

	_ = conn.Close() // best-effort; only checking reachability

	return true
}

// ResetSession sends a new_session command to clear the q daemon's conversation history.
func ResetSession() error {
	conn, err := net.DialTimeout("unix", SocketPath(), 2*time.Second)
	if err != nil {
		return fmt.Errorf("connecting to q daemon: %w", err)
	}

	defer func() { _ = conn.Close() }() // best-effort; connection errors after response are non-critical

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("setting connection deadline: %w", err)
	}

	req := ClientRequest{Type: "new_session"}
	if err := Encode(conn, req); err != nil {
		return fmt.Errorf("sending new_session: %w", err)
	}

	dec := NewDecoder(conn)
	if _, err := dec.Next(); err != nil {
		return fmt.Errorf("new_session ack: %w", err)
	}

	return nil
}
