package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/PedroKlein/tools/pkg/pirpc"
)

// Daemon manages a persistent pi --mode rpc subprocess and a Unix socket
// for accepting client connections. The daemon runs indefinitely — idle
// timeout only resets the session, never shuts down the process.
type Daemon struct {
	cfg Config

	socketPath string
	pidPath    string

	cmd   *exec.Cmd
	piIn  io.WriteCloser
	piOut io.ReadCloser

	listener net.Listener
	mu       sync.Mutex // serializes client access to pi
	idle     *time.Timer

	done chan struct{}
}

// SocketPath returns the default socket path for the current user.
func SocketPath() string {
	return pirpc.SocketPath()
}

// PIDPath returns the default PID file path.
func PIDPath() string {
	return pirpc.PIDPath()
}

// PiCommand returns the command name for the pi binary.
// Uses Q_PI_CMD env var if set (for testing).
func PiCommand() string {
	if cmd := os.Getenv("Q_PI_CMD"); cmd != "" {
		return cmd
	}

	return "pi"
}

// BuildPiArgs constructs the argument list for spawning pi in RPC mode.
// Always uses BaseSystemPrompt. --no-session is ALWAYS included.
func BuildPiArgs(cfg Config, profile Profile) []string {
	args := []string{"--mode", "rpc"}

	// Model
	if profile.Model != "" {
		args = append(args, "--model", profile.Model)
	}

	// System prompt — always the base prompt (profiles modify per-request)
	args = append(args, "--system-prompt", BaseSystemPrompt)

	// Global pi flags from config
	args = append(args, cfg.PiFlags...)

	// Profile-specific pi flags (e.g. --thinking for think profile)
	args = append(args, profile.PiFlags...)

	// Ensure --no-session is present (belt and suspenders)
	if !slices.Contains(args, "--no-session") {
		args = append(args, "--no-session")
	}

	return args
}

// NewDaemon creates a new daemon instance.
func NewDaemon(cfg Config) *Daemon {
	return &Daemon{
		cfg:        cfg,
		socketPath: SocketPath(),
		pidPath:    PIDPath(),
		done:       make(chan struct{}),
	}
}

// Run starts the daemon: spawns pi, listens on socket, handles connections.
// Blocks until explicit shutdown (SIGTERM/SIGINT or q stop).
func (d *Daemon) Run() error {
	// Ensure socket directory exists
	if err := os.MkdirAll(filepath.Dir(d.socketPath), 0o700); err != nil {
		return fmt.Errorf("creating socket dir: %w", err)
	}

	// Check for existing daemon
	if isSocketAlive(d.socketPath) {
		return fmt.Errorf("daemon already running (socket: %s)", d.socketPath)
	}

	// Clean stale socket
	_ = os.Remove(d.socketPath)

	// Resolve default profile for model/piFlags
	profile, err := ResolveProfile(d.cfg, d.cfg.DefaultProfile)
	if err != nil {
		return fmt.Errorf("resolving profile: %w", err)
	}

	// Start pi subprocess
	if startErr := d.startPi(profile); startErr != nil {
		return fmt.Errorf("starting pi: %w", startErr)
	}

	// Listen on Unix socket
	ln, err := net.Listen("unix", d.socketPath)
	if err != nil {
		d.killPi()
		return fmt.Errorf("listen: %w", err)
	}

	d.listener = ln

	// Write PID file
	if err := os.WriteFile(d.pidPath, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		d.cleanup()
		return fmt.Errorf("writing pid file: %w", err)
	}

	// Set up idle timer — resets session on idle, does NOT shut down
	d.idle = time.AfterFunc(d.cfg.IdleTimeoutDuration(), d.idleReset)

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		d.Shutdown()
	}()

	// Accept connections
	go d.acceptLoop()

	// Wait for shutdown
	<-d.done

	return nil
}

// Shutdown gracefully stops the daemon.
func (d *Daemon) Shutdown() {
	select {
	case <-d.done:
		return
	default:
	}

	close(d.done)
	d.cleanup()
}

func (d *Daemon) idleReset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	msg := NewSessionMsg{Type: "new_session", ID: generateID()}
	if err := Encode(d.piIn, msg); err != nil {
		fmt.Fprintf(os.Stderr, "q daemon: idle reset failed: %v\n", err)
	}
}

func (d *Daemon) acceptLoop() {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			select {
			case <-d.done:
				return
			default:
				return
			}
		}

		go d.handleConnection(conn)
	}
}

func (d *Daemon) handleConnection(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	// Reset idle timer on any activity
	d.idle.Reset(d.cfg.IdleTimeoutDuration())

	// Serialize access to pi subprocess
	d.mu.Lock()
	defer d.mu.Unlock()

	// Read the request line from client
	req, err := ReadClientRequest(conn)
	if err != nil {
		writeError(conn, "reading request: "+err.Error())
		return
	}

	switch req.Type {
	case "new_session":
		d.handleNewSession(conn)
	case "prompt":
		d.handlePrompt(conn, req.Message)
	case "ping":
		writeJSON(conn, map[string]string{"type": "pong"})
	default:
		writeError(conn, "unknown message type: "+req.Type)
	}
}

func (d *Daemon) handleNewSession(conn net.Conn) {
	msg := NewSessionMsg{Type: "new_session", ID: generateID()}
	if err := Encode(d.piIn, msg); err != nil {
		writeError(conn, "sending new_session to pi: "+err.Error())
		return
	}

	writeJSON(conn, map[string]any{"type": "response", "success": true})
}

func (d *Daemon) handlePrompt(conn net.Conn, message string) {
	msg := PromptMsg{
		Type:    "prompt",
		Message: message,
		ID:      generateID(),
	}
	if err := Encode(d.piIn, msg); err != nil {
		writeError(conn, "sending prompt to pi: "+err.Error())
		return
	}

	// Stream pi's output to client until agent_end
	piDec := NewDecoder(d.piOut)

	for {
		piEvent, err := piDec.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				writeError(conn, "pi process ended unexpectedly")
			} else {
				writeError(conn, "reading pi output: "+err.Error())
			}

			return
		}

		if err := Encode(conn, piEvent); err != nil {
			return // client disconnected
		}

		if piEvent.IsAgentEnd() {
			return
		}
	}
}

func (d *Daemon) startPi(profile Profile) error {
	piCmd := PiCommand()
	args := BuildPiArgs(d.cfg, profile)

	d.cmd = exec.Command(piCmd, args...)
	d.cmd.Stderr = os.Stderr

	var err error

	d.piIn, err = d.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	d.piOut, err = d.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := d.cmd.Start(); err != nil {
		return fmt.Errorf("starting pi: %w", err)
	}

	return nil
}

func (d *Daemon) killPi() {
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Process.Signal(syscall.SIGTERM)
		_ = d.cmd.Wait()
	}
}

func (d *Daemon) cleanup() {
	if d.idle != nil {
		d.idle.Stop()
	}

	if d.listener != nil {
		_ = d.listener.Close()
	}

	d.killPi()
	_ = os.Remove(d.socketPath)
	_ = os.Remove(d.pidPath)
}

// isSocketAlive checks if a daemon is running by attempting a connection.
func isSocketAlive(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return false
	}

	_ = Encode(conn, map[string]string{"type": "ping"})
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	_ = conn.Close()

	if err != nil {
		return false
	}

	return strings.Contains(string(buf[:n]), "pong")
}

func writeError(conn net.Conn, msg string) {
	_ = Encode(conn, map[string]any{"type": "error", "error": msg})
}

func writeJSON(conn net.Conn, v any) {
	_ = Encode(conn, v)
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)

	return hex.EncodeToString(b)
}
