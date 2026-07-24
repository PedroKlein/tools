package main

// Exit codes for agent-friendly error differentiation.
// Matches the convention used by cmd/repos, cmd/pia, cmd/q, cmd/todo.
const (
	ExitOK          = 0 // Success
	ExitError       = 1 // General error (IO, hook failure, invalid config)
	ExitAmbiguous   = 2 // Query matched multiple themes / semantic ambiguity
	ExitNotFound    = 3 // Theme not installed / previous_theme references removed
)
