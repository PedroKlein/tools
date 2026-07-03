package main

// Exit codes for agent-friendly error differentiation.
const (
	ExitOK        = 0 // Success
	ExitError     = 1 // General error (IO, git failure, invalid args)
	ExitAmbiguous = 2 // Query matched multiple repos
	ExitNotFound  = 3 // Query matched zero repos
)
