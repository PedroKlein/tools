package reload

import "context"

// hookGhostty ports .hooks/ghostty.sh to Go.
//
// The shell version was a documented no-op after the transparency
// investigation (see git history of ghostty.sh):
//
//   - On Ghostty 1.3.1 + macOS + window-decoration=false, reload_config
//     DESTROYS transparency in the target window. Every theme swap made
//     open windows fully opaque — worse than not reloading.
//   - The osc-broadcast hook already sends OSC 4/10/11/12 to every open
//     Ghostty PTY. Palette/fg/bg/cursor retint live without reload_config.
//   - background-opacity/background-blur cannot hot-reload on Ghostty
//     1.3.1 macOS regardless (upstream limitation). New windows pick up
//     new values on launch.
//
// This Go port preserves the same semantics: no-op. When Ghostty upstream
// ships a signal-based reload API that doesn't drop transparency (issues
// 12922, 13324, 9206, PR 5083), swap the body for the signal send.
//
// Linux SIGUSR2 was considered but Ghostty does not (yet) handle it as
// a reload trigger — sending SIGUSR2 to a process that doesn't handle
// it terminates the process on Linux. Silent no-op is safer than
// termination.
func hookGhostty(_ context.Context, _ string) error {
	// Reserved for future signal-based reload once upstream lands one.
	return nil
}
