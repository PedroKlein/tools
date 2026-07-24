package reload

// Signal aliases hide the syscall.Signal type behind a package-local alias
// so run.go, hooks.go, and tests don't sprinkle syscall imports.
//
// runtime detail: syscall.Signal values differ per-GOOS but map to the
// same POSIX numbers on all our target platforms (linux, darwin). No
// need for a build-tag split.

import "syscall"

type syscallSignal = syscall.Signal

var (
	sigUSR1 = syscall.SIGUSR1
	sigUSR2 = syscall.SIGUSR2
	sigHUP  = syscall.SIGHUP
	sigTERM = syscall.SIGTERM
)
