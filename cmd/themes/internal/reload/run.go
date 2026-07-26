package reload

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Result reports one hook's execution outcome.
type Result struct {
	Name     string
	Skipped  bool
	Err      error
	Duration time.Duration
}

// Options tunes RunAll.
type Options struct {
	// SkipHooks holds names to omit. If nil, reads THEME_SKIP_HOOKS.
	SkipHooks map[string]bool

	// LiveApply overrides the default (false = commit). Rare; used by tests.
	LiveApply *bool

	// PerHookTimeout overrides the default 4s cap. Zero uses defaultTimeout.
	PerHookTimeout time.Duration

	// Verbose prints per-hook status to os.Stderr.
	Verbose bool

	// Stderr redirects per-hook stderr from external scripts. Defaults to
	// os.Stderr when nil. Callers running under a TUI can pass a file to
	// keep hook errors from corrupting the visible frame.
	Stderr io.Writer
}

// RunAll runs every registered hook against themeDir concurrently.
//
// Errors are collected but never propagated as a failure — a broken hook
// (e.g. sketchybar not installed) should never block the rest of a swap.
// Callers who care about specific hooks can inspect the returned slice.
//
// Stale-reload guard: since D3 released the flock before the reload phase,
// a concurrent Set() may have already swapped .current to a NEWER theme by
// the time we start firing hooks. If .current no longer resolves to
// themeDir, we abort — the newer swap's own RunAll will handle the retint.
// Without this guard, an older reload finishing after a newer swap would
// overwrite pi.json, ghostty.conf, tmux status, etc. with stale colors.
func RunAll(ctx context.Context, themeDir string, opts Options) []Result {
	skip := opts.SkipHooks
	if skip == nil {
		skip = SkipList()
	}
// live defaults to false (commit-mode) unless the caller explicitly
// overrides it. Historically we also read THEME_LIVE_APPLY from the
// environment; a4 removed that plumbing because Set is now the sole
// LiveApply-parameter carrier.
	live := false
	if opts.LiveApply != nil {
		live = *opts.LiveApply
	}
	hooks := FilterHooks(skip, live)
	timeout := opts.PerHookTimeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	// Stale-reload guard.
	if isStaleReload(themeDir) {
		if opts.Verbose {
			fmt.Fprintf(stderr, "reload: skip (superseded by newer theme swap)\n")
		}
		return nil
	}

	// Also surface skipped hooks in verbose mode so operators can see
	// what the picker's live-apply mode elided.
	if opts.Verbose {
		for _, h := range Registry() {
			if skip[h.Name] {
				fmt.Fprintf(stderr, "reload: %s skip (user-requested)\n", h.Name)
			}
		}
	}

	results := make([]Result, len(hooks))
	var wg sync.WaitGroup
	for i, h := range hooks {
		wg.Add(1)
		go func(i int, h Hook) {
			defer wg.Done()
			start := time.Now()
			hctx, cancel := context.WithTimeout(ctx, hookTimeout(h, timeout))
			defer cancel()

			err := runOne(hctx, h, themeDir, stderr)
			results[i] = Result{
				Name:     h.Name,
				Err:      err,
				Duration: time.Since(start),
			}
			if opts.Verbose {
				if err != nil {
					fmt.Fprintf(stderr, "reload: %s fail (%v): %v\n", h.Name, results[i].Duration, err)
				} else {
					fmt.Fprintf(stderr, "reload: %s ok (%v)\n", h.Name, results[i].Duration)
				}
			}
		}(i, h)
	}
	wg.Wait()
	return results
}

// hookTimeout returns the hook-specific cap or the fallback.
func hookTimeout(h Hook, fallback time.Duration) time.Duration {
	if h.Timeout > 0 {
		return h.Timeout
	}
	return fallback
}

// runOne dispatches the hook. New-shape hooks (any of
// RunPreview/RunCommit/OS set) always route through h.Fn. Legacy hooks
// still switch on Kind. Returns nil on success (including "no target
// process" for KindSignal — signalling nothing is fine).
func runOne(ctx context.Context, h Hook, themeDir string, stderr io.Writer) error {
	// b0-forward path: new-shape hooks are pure Go functions.
	if h.hasNewShape() {
		if h.Fn == nil {
			return fmt.Errorf("new-shape hook %s has nil Fn", h.Name)
		}
		return runFn(ctx, h, themeDir)
	}
	// Legacy Kind switch.
	switch h.Kind {
	case KindNoop:
		return nil
	case KindCommand:
		return runCommand(ctx, h, stderr)
	case KindSignal:
		return runSignal(h)
	case KindInline:
		if h.Fn == nil {
			return fmt.Errorf("inline hook %s has nil Fn", h.Name)
		}
		return runFn(ctx, h, themeDir)
	case KindExternal:
		return runExternal(ctx, h, themeDir, stderr)
	}
	return fmt.Errorf("unknown hook kind %d for %s", h.Kind, h.Name)
}

// runFn runs h.Fn under the context deadline. Fn implementations are
// required to honor ctx.Done() themselves (see reload.Hook doc). We
// still run under a select so a misbehaving Fn doesn't block RunAll's
// overall timeout — but the goroutine may leak past deadline. In
// practice all in-tree Fns return in milliseconds, so this is a bounded
// soft-leak, not a growing one.
func runFn(ctx context.Context, h Hook, themeDir string) error {
	done := make(chan error, 1)
	go func() { done <- h.Fn(ctx, themeDir) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("%s: %w", h.Name, ctx.Err())
	}
}

// runCommand runs Cmd + Args under the context deadline.
//
// If the command binary is missing (LookPath fails), returns nil so we
// don't spam errors for optional tools like sketchybar on Linux.
func runCommand(ctx context.Context, h Hook, stderr io.Writer) error {
	if _, err := exec.LookPath(h.Cmd); err != nil {
		return nil // missing binary is fine
	}
	cmd := exec.CommandContext(ctx, h.Cmd, h.Args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		// Timeout leaves the goroutine stuck; deadline exceeded returns.
		if ctx.Err() != nil {
			return fmt.Errorf("%s: %w", h.Name, ctx.Err())
		}
		return fmt.Errorf("%s: %w", h.Name, err)
	}
	return nil
}

// runSignal fans out a signal to every running process matching Target.
func runSignal(h Hook) error {
	sig := parseSignal(h.Signal)
	return signalProcess(h.SignalTarget, sig)
}

// runExternal invokes the .sh file with themeDir as its argv[1].
func runExternal(ctx context.Context, h Hook, themeDir string, stderr io.Writer) error {
	script := hookScript(h.Script)
	if _, err := os.Stat(script); err != nil {
		return nil // absent scripts are fine (fresh install)
	}
	cmd := exec.CommandContext(ctx, script, themeDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	// Propagate the LIVE_APPLY signal so downstream scripts can react.
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%s: timed out after %v", h.Name, hookTimeout(h, defaultTimeout))
		}
		return fmt.Errorf("%s: %w", h.Name, err)
	}
	return nil
}

// parseSignal maps a signal name string (e.g. "SIGUSR1") to a syscall.Signal.
// Falls back to SIGHUP for unknown names — safest reload default.
func parseSignal(name string) syscallSignal {
	switch strings.ToUpper(name) {
	case "SIGUSR1":
		return sigUSR1
	case "SIGUSR2":
		return sigUSR2
	case "SIGHUP":
		return sigHUP
	case "SIGTERM":
		return sigTERM
	}
	return sigHUP
}
