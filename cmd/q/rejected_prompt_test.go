package main_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCLI_RejectedPromptReturnsErrorWithoutHanging(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "q")

	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building q: %v\n%s", err, output)
	}

	mockPi, err := filepath.Abs("testdata/reject-pi.sh")
	if err != nil {
		t.Fatal(err)
	}

	env := append(os.Environ(),
		"XDG_RUNTIME_DIR="+dir,
		"Q_CONFIG_PATH="+filepath.Join(dir, "missing.json"),
		"Q_PI_CMD="+mockPi,
	)
	daemon := exec.Command(binary, "start")

	daemon.Env = env
	if startErr := daemon.Start(); startErr != nil {
		t.Fatal(startErr)
	}

	t.Cleanup(func() {
		_ = daemon.Process.Signal(syscall.SIGTERM)
		_ = daemon.Wait()
	})

	deadline := time.Now().Add(3 * time.Second)

	for {
		if _, statErr := os.Stat(filepath.Join(dir, "q.pid")); statErr == nil {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("daemon did not start")
		}

		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	prompt := exec.CommandContext(ctx, binary, "hey")
	prompt.Env = env

	output, err := prompt.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("q hey hung after Pi rejected the prompt; output: %s", output)
	}

	if err == nil || !strings.Contains(string(output), "No API key found for github-copilot.") {
		t.Fatalf("expected nonzero exit with Pi's authentication error, got %v: %s", err, output)
	}

	statusCtx, statusCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer statusCancel()

	status := exec.CommandContext(statusCtx, binary, "status")

	status.Env = env
	if output, err := status.CombinedOutput(); err != nil {
		t.Fatalf("daemon must remain responsive after rejecting a prompt: %v: %s", err, output)
	}
}
