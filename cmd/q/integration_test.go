package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestIntegration_DaemonLifecycle tests the full daemon flow with a mock pi.
//
//nolint:gocognit,gocyclo // integration test with full lifecycle: setup, multiple subtests, teardown, cleanup checks
func TestIntegration_DaemonLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Set up isolated environment
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "q.sock")
	pidPath := filepath.Join(tmpDir, "q.pid")

	// Point to mock pi
	mockPi, err := filepath.Abs("testdata/mock-pi.sh")
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("Q_PI_CMD", mockPi)

	// Create config for short idle timeout
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfgData := `{"idleTimeout": "30s", "defaultProfile": "cmd"}`
	if writeErr := os.WriteFile(cfgPath, []byte(cfgData), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}

	t.Setenv("Q_CONFIG_PATH", cfgPath)

	// Load config
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Create daemon
	d := NewDaemon(cfg)
	d.socketPath = socketPath
	d.pidPath = pidPath

	// Start daemon in background
	daemonErr := make(chan error, 1)

	go func() {
		daemonErr <- d.Run()
	}()

	// Wait for socket to appear
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	// Verify socket exists
	if _, err := os.Stat(socketPath); err != nil {
		t.Fatalf("socket not created: %v", err)
	}

	// Verify PID file exists
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("PID file not created: %v", err)
	}

	// Test 1: Send a prompt
	t.Run("prompt", func(t *testing.T) {
		conn, err := net.DialTimeout("unix", socketPath, time.Second)
		if err != nil {
			t.Fatalf("connecting: %v", err)
		}
		defer conn.Close()

		// Send prompt
		req := ClientRequest{Type: "prompt", Message: "hello"}
		if encErr := Encode(conn, req); encErr != nil {
			t.Fatalf("sending: %v", encErr)
		}

		// Read response events
		dec := NewDecoder(conn)

		var deltas []string

		gotEnd := false

		for !gotEnd {
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))

			event, err := dec.Next()
			if err != nil {
				t.Fatalf("reading event: %v", err)
			}

			if event.IsTextDelta() {
				deltas = append(deltas, event.AssistantMessageEvent.Delta)
			}

			if event.IsAgentEnd() {
				gotEnd = true
			}
		}

		response := strings.Join(deltas, "")
		if !strings.Contains(response, "hello") {
			t.Errorf("expected response to contain 'hello', got %q", response)
		}
	})

	// Test 2: Ping
	t.Run("ping", func(t *testing.T) {
		conn, err := net.DialTimeout("unix", socketPath, time.Second)
		if err != nil {
			t.Fatalf("connecting: %v", err)
		}
		defer conn.Close()

		req := ClientRequest{Type: "ping"}
		if encErr := Encode(conn, req); encErr != nil {
			t.Fatalf("sending: %v", encErr)
		}

		dec := NewDecoder(conn)
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))

		event, err := dec.Next()
		if err != nil {
			t.Fatalf("reading pong: %v", err)
		}

		if event.Type != "pong" {
			t.Errorf("expected pong, got %s", event.Type)
		}
	})

	// Test 3: New session
	t.Run("new_session", func(t *testing.T) {
		conn, err := net.DialTimeout("unix", socketPath, time.Second)
		if err != nil {
			t.Fatalf("connecting: %v", err)
		}
		defer conn.Close()

		req := ClientRequest{Type: "new_session"}
		if encErr := Encode(conn, req); encErr != nil {
			t.Fatalf("sending: %v", encErr)
		}

		dec := NewDecoder(conn)
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))

		event, err := dec.Next()
		if err != nil {
			t.Fatalf("reading response: %v", err)
		}

		if !event.IsResponse() {
			t.Errorf("expected response event, got %s", event.Type)
		}
	})

	// Shutdown
	d.Shutdown()

	// Wait for daemon to exit
	select {
	case err := <-daemonErr:
		if err != nil {
			t.Fatalf("daemon error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not exit within timeout")
	}

	// Verify cleanup
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("socket not cleaned up after shutdown")
	}

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file not cleaned up after shutdown")
	}
}

// TestIntegration_DuplicateDaemon verifies second daemon detects existing one.
func TestIntegration_DuplicateDaemon(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "q.sock")
	pidPath := filepath.Join(tmpDir, "q.pid")

	mockPi, _ := filepath.Abs("testdata/mock-pi.sh")
	t.Setenv("Q_PI_CMD", mockPi)
	t.Setenv("Q_CONFIG_PATH", "/nonexistent")

	cfg := DefaultConfig()
	cfg.IdleTimeout = "30s"

	d1 := NewDaemon(cfg)
	d1.socketPath = socketPath
	d1.pidPath = pidPath

	go d1.Run()
	defer d1.Shutdown()

	// Wait for socket
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	// Try starting a second daemon
	d2 := NewDaemon(cfg)
	d2.socketPath = socketPath
	d2.pidPath = pidPath

	err := d2.Run()
	if err == nil {
		t.Error("expected error for duplicate daemon, got nil")
		d2.Shutdown()
	} else if !strings.Contains(err.Error(), "already running") {
		t.Errorf("expected 'already running' error, got: %v", err)
	}
}
